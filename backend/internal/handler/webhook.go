package handler

import (
	"net/http"
	"strconv"

	"backend/internal/model"
	"backend/internal/repo"
	"backend/internal/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// WebhookHandler handles webhook subscription endpoints.
type WebhookHandler struct {
	repo   repo.WebhookRepo
	logger *zap.Logger
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(r repo.WebhookRepo, l *logger.Logger) *WebhookHandler {
	return &WebhookHandler{
		repo:   r,
		logger: l.Named("webhook-handler"),
	}
}

// RegisterRoutes registers the webhook routes under the provided router group.
func (h *WebhookHandler) RegisterRoutes(g *gin.RouterGroup) {
	webhooks := g.Group("/webhooks")
	{
		webhooks.POST("", h.Create)
		webhooks.GET("", h.List)
		webhooks.DELETE("/:id", h.Delete)
	}
}

// ---------- Request / Response payloads ----------

// CreateWebhookRequest represents the payload for creating a webhook subscription.
type CreateWebhookRequest struct {
	URL    string   `json:"url" binding:"required,url"`
	Secret string   `json:"secret" binding:"required,min=32"` // minimum 32 characters for HMAC secret
	Events []string `json:"events,omitempty"`                // optional list of events to subscribe to
}

// WebhookResponse is the public representation of a webhook subscription.
type WebhookResponse struct {
	ID        int64    `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events,omitempty"`
	CreatedAt string   `json:"created_at"` // ISO8601 timestamp
}

// ---------- Handlers ----------

// Create handles POST /api/v1/webhooks
// It validates the request, hashes the secret, stores the subscription and returns the created resource.
func (h *WebhookHandler) Create(c *gin.Context) {
	var req CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid create webhook request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Retrieve authenticated user ID from context (set by auth middleware)
	uid, ok := c.Get("userID")
	if !ok {
		h.logger.Warn("user ID missing in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	userID, ok := uid.(int64)
	if !ok {
		h.logger.Error("userID type assertion failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Hash the secret using bcrypt before persisting.
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(req.Secret), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash webhook secret", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not process secret"})
		return
	}

	sub := &model.WebhookSubscription{
		UserID:     userID,
		URL:        req.URL,
		SecretHash: string(hashedSecret),
		Events:     req.Events,
	}

	if err := h.repo.Create(c.Request.Context(), sub); err != nil {
		h.logger.Error("failed to store webhook subscription", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create webhook subscription"})
		return
	}

	resp := WebhookResponse{
		ID:        sub.ID,
		URL:       sub.URL,
		Events:    sub.Events,
		CreatedAt: sub.CreatedAt.UTC().Format(http.TimeFormat),
	}
	c.JSON(http.StatusCreated, resp)
}

// List handles GET /api/v1/webhooks
// Returns all webhook subscriptions belonging to the authenticated user.
func (h *WebhookHandler) List(c *gin.Context) {
	uid, ok := c.Get("userID")
	if !ok {
		h.logger.Warn("user ID missing in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	userID, ok := uid.(int64)
	if !ok {
		h.logger.Error("userID type assertion failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	subs, err := h.repo.ListByUser(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list webhook subscriptions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve webhooks"})
		return
	}

	resp := make([]WebhookResponse, 0, len(subs))
	for _, s := range subs {
		resp = append(resp, WebhookResponse{
			ID:        s.ID,
			URL:       s.URL,
			Events:    s.Events,
			CreatedAt: s.CreatedAt.UTC().Format(http.TimeFormat),
		})
	}
	c.JSON(http.StatusOK, resp)
}

// Delete handles DELETE /api/v1/webhooks/:id
// Removes a webhook subscription belonging to the authenticated user.
func (h *WebhookHandler) Delete(c *gin.Context) {
	uid, ok := c.Get("userID")
	if !ok {
		h.logger.Warn("user ID missing in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	userID, ok := uid.(int64)
	if !ok {
		h.logger.Error("userID type assertion failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	idParam := c.Param("id")
	subID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || subID <= 0 {
		h.logger.Warn("invalid webhook ID", zap.String("id", idParam))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook ID"})
		return
	}

	// Verify ownership before deletion.
	existing, err := h.repo.GetByID(c.Request.Context(), userID, subID)
	if err != nil {
		h.logger.Error("failed to fetch webhook for deletion", zap.Int64("id", subID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete webhook"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}

	if err := h.repo.DeleteByID(c.Request.Context(), userID, subID); err != nil {
		h.logger.Error("failed to delete webhook", zap.Int64("id", subID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete webhook"})
		return
	}

	c.Status(http.StatusNoContent)
}