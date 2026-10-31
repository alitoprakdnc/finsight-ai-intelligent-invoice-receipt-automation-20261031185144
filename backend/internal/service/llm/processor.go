package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Config holds configuration for the LLM processor.
type Config struct {
	// Enabled toggles whether LLM processing is performed.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint is the HTTP URL of the Ollama/LiteLLM generate API.
	// Example: http://localhost:11434/api/generate
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Model is the name of the model to invoke, e.g. "llama2".
	Model string `json:"model" yaml:"model"`

	// Temperature controls randomness (0.0‑1.0). Optional.
	Temperature float64 `json:"temperature" yaml:"temperature"`

	// MaxTokens limits the generated token count. Optional.
	MaxTokens int `json:"max_tokens" yaml:"max_tokens"`

	// RequestTimeout is the maximum time spent waiting for the LLM service.
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`
}

// LineItem represents a single line item extracted from an invoice.
type LineItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
}

// EnrichedData is the structured result produced by the LLM.
type EnrichedData struct {
	VendorName    string     `json:"vendor_name"`
	VendorTaxID   string     `json:"vendor_tax_id"`
	InvoiceNumber string     `json:"invoice_number"`
	InvoiceDate   string     `json:"invoice_date"`
	TotalAmount   string     `json:"total_amount"`
	LineItems     []LineItem `json:"line_items"`
}

// Processor provides methods to call the local LLM service.
type Processor struct {
	client *http.Client
	cfg    Config
	logger *zap.Logger
}

// NewProcessor creates a new LLM processor instance.
func NewProcessor(cfg Config, logger *zap.Logger) *Processor {
	// Ensure sensible defaults.
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.2
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 1024
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:11434/api/generate"
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Processor{
		client: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		cfg:    cfg,
		logger: logger,
	}
}

// Process sends the raw OCR text to the LLM and returns enriched invoice data.
// If the processor is disabled, it returns an empty EnrichedData and no error.
func (p *Processor) Process(ctx context.Context, rawOCR string) (EnrichedData, error) {
	if !p.cfg.Enabled {
		p.logger.Debug("LLM processing disabled; returning empty result")
		return EnrichedData{}, nil
	}

	// Build the prompt.
	prompt := buildPrompt(rawOCR)

	// Prepare request payload.
	reqPayload := map[string]interface{}{
		"model":       p.cfg.Model,
		"prompt":      prompt,
		"stream":      false,
		"temperature": p.cfg.Temperature,
		"max_tokens":  p.cfg.MaxTokens,
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		p.logger.Error("failed to marshal LLM request payload", zap.Error(err))
		return EnrichedData{}, fmt.Errorf("marshal llm request: %w", err)
	}

	// Create HTTP request with context.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		p.logger.Error("failed to create LLM request", zap.Error(err))
		return EnrichedData{}, fmt.Errorf("create llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Perform request.
	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("LLM request failed", zap.Error(err))
		return EnrichedData{}, fmt.Errorf("llm request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.logger.Error("LLM service returned non‑200 status",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return EnrichedData{}, fmt.Errorf("llm service error: %s", resp.Status)
	}

	// Decode Ollama response.
	var llmResp struct {
		Model    string `json:"model"`
		Created  string `json:"created_at"`
		Response string `json:"response"` // The generated text.
		Done     bool   `json:"done"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		p.logger.Error("failed to decode LLM response", zap.Error(err))
		return EnrichedData{}, fmt.Errorf("decode llm response: %w", err)
	}
	if !llmResp.Done {
		p.logger.Warn("LLM response indicated incomplete generation")
	}

	// The model should return pure JSON. Trim whitespace just in case.
	rawJSON := bytes.TrimSpace([]byte(llmResp.Response))

	var enriched EnrichedData
	if err := json.Unmarshal(rawJSON, &enriched); err != nil {
		// Provide a more helpful error including the raw response.
		p.logger.Error("failed to unmarshal enriched data from LLM response",
			zap.Error(err),
			zap.String("raw_response", string(llmResp.Response)),
		)
		return EnrichedData{}, fmt.Errorf("unmarshal enriched data: %w", err)
	}

	p.logger.Debug("LLM enrichment successful", zap.Any("enriched", enriched))
	return enriched, nil
}

// buildPrompt creates the instruction sent to the LLM.
// It asks the model to output a strict JSON object matching EnrichedData.
func buildPrompt(rawOCR string) string {
	const tmpl = `You are an expert invoice data extractor. 
Given the raw OCR text of an invoice, extract the following fields and output **only** a JSON object with this exact shape (no extra text, no markdown):

{
  "vendor_name": string,
  "vendor_tax_id": string,
  "invoice_number": string,
  "invoice_date": string (ISO 8601, e.g., "2023-07-15"),
  "total_amount": string (currency, e.g., "USD 1,234.56"),
  "line_items": [
    {
      "description": string,
      "quantity": number,
      "unit_price": number,
      "amount": number
    },
    ...
  ]
}

If any field cannot be determined, set its value to null.

OCR Text:
"""%s"""
`
	return fmt.Sprintf(tmpl, rawOCR)
}

// HealthCheck pings the LLM endpoint to verify availability.
// It returns nil if the service responds with HTTP 200 within the configured timeout.
func (p *Processor) HealthCheck(ctx context.Context) error {
	if !p.cfg.Enabled {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("create healthcheck request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("llm healthcheck request failed: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llm healthcheck returned status %d", resp.StatusCode)
	}
	return nil
}