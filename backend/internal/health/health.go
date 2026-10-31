package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

// ComponentStatus represents the health of a single subsystem.
type ComponentStatus struct {
	Name       string `json:"name"`
	Healthy    bool   `json:"healthy"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// HealthResponse aggregates the status of all components.
type HealthResponse struct {
	Status     string            `json:"status"` // "healthy" or "unhealthy"
	Components []ComponentStatus `json:"components"`
	Timestamp  time.Time         `json:"timestamp"`
}

// defaultTimeout is the maximum time a single health check may take.
const defaultTimeout = 2 * time.Second

// checkComponent runs a health‑check function with a timeout and records the result.
func checkComponent(ctx context.Context, name string, checkFn func(context.Context) error) ComponentStatus {
	if checkFn == nil {
		return ComponentStatus{
			Name:    name,
			Healthy: false,
			Error:   "health check not configured",
		}
	}

	start := time.Now()
	cctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	err := checkFn(cctx)
	duration := time.Since(start)

	cs := ComponentStatus{
		Name:       name,
		Healthy:    err == nil,
		DurationMs: duration.Milliseconds(),
	}
	if err != nil {
		cs.Error = err.Error()
	}
	return cs
}

// CheckAll runs health checks for the core services used by FinSight AI.
// The caller supplies concrete check functions for OCR, LLM and the task queue.
// If a check function is nil the component will be reported as unhealthy.
func CheckAll(
	ctx context.Context,
	db *sql.DB,
	redisClient *redis.Client,
	ocrCheck func(context.Context) error,
	llmCheck func(context.Context) error,
	queueCheck func(context.Context) error,
) HealthResponse {

	components := []ComponentStatus{
		// Database health – simple ping.
		checkComponent(ctx, "postgresql", func(c context.Context) error {
			return db.PingContext(c)
		}),
		// Redis health – PING command.
		checkComponent(ctx, "redis", func(c context.Context) error {
			return redisClient.Ping(c).Err()
		}),
		// OCR service health.
		checkComponent(ctx, "ocr", ocrCheck),
		// LLM service health.
		checkComponent(ctx, "llm", llmCheck),
		// Task‑queue health.
		checkComponent(ctx, "queue", queueCheck),
	}

	overall := "healthy"
	for _, cs := range components {
		if !cs.Healthy {
			overall = "unhealthy"
			break
		}
	}

	return HealthResponse{
		Status:     overall,
		Components: components,
		Timestamp:  time.Now().UTC(),
	}
}

// ---------------------------------------------------------------------------
// Helper health‑check implementations for common services.
// These can be passed directly to CheckAll or wrapped by higher‑level code.
// ---------------------------------------------------------------------------

// PingPostgres returns a health‑check function for a *sql.DB instance.
func PingPostgres(db *sql.DB) func(context.Context) error {
	return func(ctx context.Context) error {
		return db.PingContext(ctx)
	}
}

// PingRedis returns a health‑check function for a redis.Client instance.
func PingRedis(rdb *redis.Client) func(context.Context) error {
	return func(ctx context.Context) error {
		return rdb.Ping(ctx).Err()
	}
}

// HTTPPing returns a health‑check function that performs a GET request
// against the supplied URL and expects a 2xx response.
// It is useful for services that expose a /health endpoint (OCR, LLM, queue, …).
func HTTPPing(url string) func(context.Context) error {
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		// Short client with timeout enforced by the context.
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// JSON utilities – used by the health handler to serialize the response.
// ---------------------------------------------------------------------------

// MarshalJSON returns the JSON representation of a HealthResponse.
// It never returns an error; in the unlikely case of a marshal failure,
// it falls back to a minimal JSON payload.
func (hr HealthResponse) MarshalJSON() ([]byte, error) {
	type alias HealthResponse // avoid recursion
	b, err := json.Marshal(alias(hr))
	if err != nil {
		// Fallback – should never happen because all fields are marshal‑able.
		fallback := fmt.Sprintf(`{"status":"%s","components":[],"timestamp":"%s"}`, hr.Status, hr.Timestamp.Format(time.RFC3339))
		return []byte(fallback), nil
	}
	return b, nil
}

// ---------------------------------------------------------------------------
// Convenience wrappers for the Gin handler (optional but handy).
// ---------------------------------------------------------------------------

type HealthChecker struct {
	DB        *sql.DB
	Redis     *redis.Client
	OCRURL    string // optional; if empty OCR health is skipped
	LLMURL    string // optional; if empty LLM health is skipped
	QueueURL  string // optional; if empty queue health is skipped
}

// New creates a HealthChecker ready to be used by a Gin route.
func New(db *sql.DB, redisClient *redis.Client, ocrURL, llmURL, queueURL string) *HealthChecker {
	return &HealthChecker{
		DB:       db,
		Redis:    redisClient,
		OCRURL:   ocrURL,
		LLMURL:   llmURL,
		QueueURL: queueURL,
	}
}

// Run executes all checks and returns the aggregated response.
func (hc *HealthChecker) Run(ctx context.Context) HealthResponse {
	var ocrFn, llmFn, queueFn func(context.Context) error

	if hc.OCRURL != "" {
		ocrFn = HTTPPing(hc.OCRURL)
	}
	if hc.LLMURL != "" {
		llmFn = HTTPPing(hc.LLMURL)
	}
	if hc.QueueURL != "" {
		queueFn = HTTPPing(hc.QueueURL)
	}

	return CheckAll(ctx, hc.DB, hc.Redis, ocrFn, llmFn, queueFn)
}

// ---------------------------------------------------------------------------
// Errors that callers may want to inspect.
// ---------------------------------------------------------------------------

var (
	// ErrComponentNotConfigured is returned when a health‑check function is nil.
	ErrComponentNotConfigured = errors.New("health check not configured")
)