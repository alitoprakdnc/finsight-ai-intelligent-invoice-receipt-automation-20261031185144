package queue

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"time"

	"finSight/internal/logger"
	"finSight/internal/model"
	"finSight/internal/repo"
	"finSight/internal/service/llm"
	"finSight/internal/service/ocr"

	"github.com/streadway/amqp"
	"go.uber.org/zap"
)

// TaskPayload represents the message pushed to the queue by the API layer.
type TaskPayload struct {
	InvoiceID int64  `json:"invoice_id"`
	FileURL   string `json:"file_url"`
}

// OCRService abstracts the OCR implementation.
type OCRService interface {
	Process(ctx context.Context, fileURL string) (rawText string, confidence float64, err error)
}

// LLMService abstracts the optional LLM validation/enrichment step.
type LLMService interface {
	Process(ctx context.Context, rawText string) (map[string]any, error)
}

// Worker pulls tasks from a RabbitMQ queue, processes them and dispatches webhooks.
type Worker struct {
	amqpConn   *amqp.Connection
	amqpChan   *amqp.Channel
	queueName  string
	ocrSvc     OCRService
	llmSvc     LLMService // may be nil – optional feature
	invoiceRepo repo.InvoiceRepository
	webhookRepo repo.WebhookRepository
	httpClient *http.Client
	logger     *zap.Logger
	maxRetries int
}

// NewWorker creates a new queue worker with all required dependencies.
func NewWorker(
	amqpConn *amqp.Connection,
	queueName string,
	ocrSvc OCRService,
	llmSvc LLMService,
	invoiceRepo repo.InvoiceRepository,
	webhookRepo repo.WebhookRepository,
	maxRetries int,
) (*Worker, error) {
	ch, err := amqpConn.Channel()
	if err != nil {
		return nil, err
	}

	// Ensure the queue exists.
	_, err = ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto‑delete
		false, // exclusive
		false, // no‑wait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	return &Worker{
		amqpConn:    amqpConn,
		amqpChan:    ch,
		queueName:   queueName,
		ocrSvc:      ocrSvc,
		llmSvc:      llmSvc,
		invoiceRepo: invoiceRepo,
		webhookRepo: webhookRepo,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger:     logger.NewZap(),
		maxRetries: maxRetries,
	}, nil
}

// Start begins consuming messages. It blocks until the supplied context is cancelled.
func (w *Worker) Start(ctx context.Context) error {
	deliveries, err := w.amqpChan.Consume(
		w.queueName,
		"",    // consumer tag
		false, // auto‑ack
		false, // exclusive
		false, // no‑local
		false, // no‑wait
		nil,   // args
	)
	if err != nil {
		return err
	}

	// Run a goroutine that watches for context cancellation and closes the channel.
	go func() {
		<-ctx.Done()
		_ = w.amqpChan.Cancel("", false)
	}()

	w.logger.Info("queue worker started", zap.String("queue", w.queueName))

	for d := range deliveries {
		if ctx.Err() != nil {
			break
		}
		if err := w.handleDelivery(ctx, &d); err != nil {
			w.logger.Error("failed to handle delivery", zap.Error(err), zap.Any("delivery", d))
			// Nack without requeue – the message will be dead‑lettered if a DLX is configured.
			_ = d.Nack(false, false)
		}
	}
	w.logger.Info("queue worker stopped")
	return nil
}

// handleDelivery processes a single AMQP delivery.
func (w *Worker) handleDelivery(parentCtx context.Context, d *amqp.Delivery) error {
	var payload TaskPayload
	if err := json.Unmarshal(d.Body, &payload); err != nil {
		w.logger.Error("invalid task payload", zap.Error(err))
		return d.Ack(false) // discard malformed messages
	}

	// Extract retry count from headers (if any).
	retries := 0
	if v, ok := d.Headers["x-retries"]; ok {
		switch v := v.(type) {
		case int32:
			retries = int(v)
		case int64:
			retries = int(v)
		case int:
			retries = v
		}
	}

	// Create a per‑task context with timeout.
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	if err := w.processTask(ctx, payload); err != nil {
		w.logger.Error("task processing error",
			zap.Int("retries", retries),
			zap.Error(err),
			zap.Int64("invoice_id", payload.InvoiceID),
		)

		if retries >= w.maxRetries {
			w.logger.Error("max retries reached, moving to dead‑letter",
				zap.Int64("invoice_id", payload.InvoiceID))
			// Ack the message so it is removed; a DLX can be configured on the queue.
			return d.Ack(false)
		}

		// Exponential back‑off before re‑queueing.
		backoff := time.Duration(math.Pow(2, float64(retries))) * time.Second
		w.logger.Info("re‑queueing task with back‑off", zap.Duration("backoff", backoff))

		time.Sleep(backoff)

		// Increment retry header and republish.
		newHeaders := amqp.Table{
			"x-retries": retries + 1,
		}
		if err := w.amqpChan.Publish(
			"",          // default exchange
			w.queueName, // routing key = queue name
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				Body:         d.Body,
				Headers:      newHeaders,
				DeliveryMode: amqp.Persistent,
				Timestamp:    time.Now(),
			},
		); err != nil {
			w.logger.Error("failed to republish task", zap.Error(err))
			// Nack without requeue – message will be dead‑lettered.
			return d.Nack(false, false)
		}
		// Ack original delivery.
		return d.Ack(false)
	}

	// Success – acknowledge.
	return d.Ack(false)
}

// processTask runs OCR → optional LLM → DB update → webhook dispatch.
func (w *Worker) processTask(ctx context.Context, payload TaskPayload) error {
	// 1. Load invoice record.
	inv, err := w.invoiceRepo.GetByID(ctx, payload.InvoiceID)
	if err != nil {
		return err
	}
	if inv == nil {
		return errors.New("invoice not found")
	}

	// 2. Run OCR.
	rawText, _, err := w.ocrSvc.Process(ctx, payload.FileURL)
	if err != nil {
		return err
	}

	// 3. Optional LLM enrichment.
	var extracted map[string]any
	if w.llmSvc != nil {
		extracted, err = w.llmSvc.Process(ctx, rawText)
		if err != nil {
			return err
		}
	} else {
		// Fallback: store raw OCR text only.
		extracted = map[string]any{
			"raw_text": rawText,
		}
	}

	// 4. Persist extraction results.
	if err := w.invoiceRepo.UpdateExtraction(ctx, inv.ID, rawText, extracted); err != nil {
		return err
	}

	// 5. Dispatch webhooks for the owning user.
	if err := w.dispatchWebhooks(ctx, inv.UserID, inv.ID, extracted); err != nil {
		// Webhook failures should not block the main flow, but we log them.
		w.logger.Warn("webhook dispatch errors", zap.Error(err), zap.Int64("invoice_id", inv.ID))
	}
	return nil
}

// dispatchWebhooks sends the processed invoice to all active webhook subscriptions of a user.
func (w *Worker) dispatchWebhooks(ctx context.Context, userID, invoiceID int64, payload map[string]any) error {
	subs, err := w.webhookRepo.ListByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}

	bodyBytes, err := json.Marshal(map[string]any{
		"invoice_id": invoiceID,
		"data":       payload,
	})
	if err != nil {
		return err
	}

	for _, sub := range subs {
		if !sub.Enabled {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, nil)
		if err != nil {
			w.logger.Error("failed to create webhook request", zap.Error(err), zap.String("url", sub.URL))
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Body = http.NoBody
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
		// Sign payload.
		signature := signHMAC(sub.Secret, bodyBytes)
		req.Header.Set("X-Signature", signature)

		resp, err := w.httpClient.Do(req)
		if err != nil {
			w.logger.Error("webhook delivery failed", zap.Error(err), zap.String("url", sub.URL))
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			w.logger.Warn("webhook returned non‑2xx status",
				zap.Int("status", resp.StatusCode),
				zap.String("url", sub.URL))
		}
	}
	return nil
}

// signHMAC creates a constant‑time HMAC‑SHA256 signature for webhook payloads.
func signHMAC(secret string, data []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}