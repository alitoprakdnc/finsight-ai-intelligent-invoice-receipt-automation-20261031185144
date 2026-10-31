package handler

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"github.com/FinSightAI/fin-sight/backend/internal/health"
)

// HealthHandler aggregates the dependencies required to perform health checks.
type HealthHandler struct {
	DB    *sql.DB
	Redis *redis.Client
	// The OCR, LLM and Queue health checks are encapsulated in the health package.
	// They expose a simple Check(ctx) error method.
	OCR   health.Checker
	LLM   health.Checker
	Queue health.Checker
}

// NewHealthHandler creates a new HealthHandler with the supplied dependencies.
func NewHealthHandler(db *sql.DB, rdb *redis.Client, ocr, llm, queue health.Checker) *HealthHandler {
	return &HealthHandler{
		DB:    db,
		Redis: rdb,
		OCR:   ocr,
		LLM:   llm,
		Queue: queue,
	}
}

// Register registers the health endpoint on the provided Gin router group.
func (h *HealthHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/health", h.Check)
}

// componentStatus represents the health status of an individual component.
type componentStatus struct {
	Status  string `json:"status"`            // "ok" or "unhealthy"
	Details string `json:"details,omitempty"` // optional error message when unhealthy
}

// healthResponse aggregates the status of all components.
type healthResponse struct {
	Database componentStatus `json:"database"`
	Redis    componentStatus `json:"redis"`
	OCR      componentStatus `json:"ocr"`
	LLM      componentStatus `json:"llm"`
	Queue    componentStatus `json:"queue"`
}

// Check performs health checks for all critical components and returns a JSON payload.
// If any component is unhealthy, the endpoint responds with HTTP 503 Service Unavailable.
func (h *HealthHandler) Check(c *gin.Context) {
	ctx := context.Background()
	var (
		resp       healthResponse
		allHealthy = true
	)

	// Database health check
	if err := health.CheckDB(h.DB); err != nil {
		resp.Database = componentStatus{Status: "unhealthy", Details: err.Error()}
		allHealthy = false
	} else {
		resp.Database = componentStatus{Status: "ok"}
	}

	// Redis health check
	if err := health.CheckRedis(h.Redis); err != nil {
		resp.Redis = componentStatus{Status: "unhealthy", Details: err.Error()}
		allHealthy = false
	} else {
		resp.Redis = componentStatus{Status: "ok"}
	}

	// OCR service health check
	if err := h.OCR.Check(ctx); err != nil {
		resp.OCR = componentStatus{Status: "unhealthy", Details: err.Error()}
		allHealthy = false
	} else {
		resp.OCR = componentStatus{Status: "ok"}
	}

	// LLM service health check
	if err := h.LLM.Check(ctx); err != nil {
		resp.LLM = componentStatus{Status: "unhealthy", Details: err.Error()}
		allHealthy = false
	} else {
		resp.LLM = componentStatus{Status: "ok"}
	}

	// Queue health check
	if err := h.Queue.Check(ctx); err != nil {
		resp.Queue = componentStatus{Status: "unhealthy", Details: err.Error()}
		allHealthy = false
	} else {
		resp.Queue = componentStatus{Status: "ok"}
	}

	// Choose appropriate HTTP status code
	if allHealthy {
		c.JSON(http.StatusOK, resp)
	} else {
		c.JSON(http.StatusServiceUnavailable, resp)
	}
}