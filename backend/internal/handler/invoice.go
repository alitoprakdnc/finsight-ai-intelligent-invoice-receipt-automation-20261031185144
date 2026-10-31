package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"

	"backend/internal/config"
	"backend/internal/logger"
	"backend/internal/model"
	"backend/internal/repo"
	"backend/internal/service/queue"
)

// InvoiceHandler groups all invoice‑related HTTP handlers.
type InvoiceHandler struct {
	invoiceRepo   repo.InvoiceRepo
	storageClient *minio.Client
	queueClient   queue.Client
	logger        *zap.Logger
}

// NewInvoiceHandler creates a new InvoiceHandler with its dependencies.
func NewInvoiceHandler(
	cfg *config.Config,
	invoiceRepo repo.InvoiceRepo,
	qc queue.Client,
) (*InvoiceHandler, error) {
	// Initialise MinIO (S3‑compatible) client – works for MinIO in dev and any S3 provider in prod.
	minioClient, err := minio.New(cfg.Storage.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""),
		Secure: cfg.Storage.UseSSL,
		Region: cfg.Storage.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialise storage client: %w", err)
	}

	return &InvoiceHandler{
		invoiceRepo:   invoiceRepo,
		storageClient: minioClient,
		queueClient:   qc,
		logger:        logger.New(),
	}, nil
}

// ---------------------------------------------------------------------
// Helper utilities
// ---------------------------------------------------------------------

// getUserID extracts the authenticated user ID from the Gin context.
// The auth middleware stores the ID under the key "userID" as int64.
func getUserID(c *gin.Context) (int64, error) {
	val, exists := c.Get("userID")
	if !exists {
		return 0, fmt.Errorf("user not authenticated")
	}
	uid, ok := val.(int64)
	if !ok {
		return 0, fmt.Errorf("invalid user ID type")
	}
	return uid, nil
}

// allowedMIMETypes defines the MIME types we accept for invoice uploads.
var allowedMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/tiff": true,
	"application/pdf": true,
}

// maxUploadSize limits the size of a single upload (10 MiB).
const maxUploadSize = 10 << 20 // 10 MB

// validateFile checks size and MIME type of the uploaded file.
func validateFile(header *multipart.FileHeader) error {
	if header.Size > maxUploadSize {
		return fmt.Errorf("file size exceeds %d bytes", maxUploadSize)
	}
	ctype := header.Header.Get("Content-Type")
	if !allowedMIMETypes[ctype] {
		return fmt.Errorf("unsupported content type: %s", ctype)
	}
	return nil
}

// uploadToBucket streams the multipart file to the configured bucket and returns the public URL.
func (h *InvoiceHandler) uploadToBucket(ctx context.Context, userID int64, file multipart.File, header *multipart.FileHeader) (string, error) {
	objectName := fmt.Sprintf("%d/%s%s", userID, uuid.New().String(), filepath.Ext(header.Filename))

	// Ensure the bucket exists (idempotent)
	exists, err := h.storageClient.BucketExists(ctx, h.storageClient.BucketName())
	if err != nil {
		return "", fmt.Errorf("bucket existence check failed: %w", err)
	}
	if !exists {
		if err := h.storageClient.MakeBucket(ctx, h.storageClient.BucketName(), minio.MakeBucketOptions{}); err != nil {
			return "", fmt.Errorf("bucket creation failed: %w", err)
		}
	}

	// Upload the object
	_, err = h.storageClient.PutObject(ctx, h.storageClient.BucketName(), objectName, file, header.Size, minio.PutObjectOptions{
		ContentType: header.Header.Get("Content-Type"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to storage: %w", err)
	}

	// Build a URL – for MinIO we can use the endpoint directly.
	// In production the bucket may be public or behind a CDN; adjust as needed.
	url := fmt.Sprintf("%s/%s/%s", h.storageClient.EndpointURL().String(), h.storageClient.BucketName(), objectName)
	return url, nil
}

// ---------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------

// Upload handles POST /api/v1/invoices
// Expected multipart form field: "file"
func (h *InvoiceHandler) Upload(c *gin.Context) {
	ctx := c.Request.Context()

	uid, err := getUserID(c)
	if err != nil {
		h.logger.Warn("unauthenticated upload attempt", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	// Parse multipart form (maxMemory is irrelevant because we stream directly)
	if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
		h.logger.Warn("failed to parse multipart form", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart request"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		h.logger.Warn("file not provided", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "file field is required"})
		return
	}
	if err := validateFile(fileHeader); err != nil {
		h.logger.Warn("file validation failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		h.logger.Error("cannot open uploaded file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
		return
	}
	defer file.Close()

	// Stream upload to object storage
	storageURL, err := h.uploadToBucket(ctx, uid, file, fileHeader)
	if err != nil {
		h.logger.Error("storage upload failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store file"})
		return
	}

	// Create invoice record
	inv := &model.Invoice{
		ID:        uuid.New().String(),
		UserID:    uid,
		FileURL:   storageURL,
		Status:    model.InvoiceStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.invoiceRepo.Create(ctx, inv); err != nil {
		h.logger.Error("invoice DB insert failed", zap.Error(err))
		// Attempt to clean up the uploaded object to avoid orphaned files.
		_ = h.storageClient.RemoveObject(ctx, h.storageClient.BucketName(), strings.TrimPrefix(storageURL, fmt.Sprintf("%s/%s/", h.storageClient.EndpointURL().String(), h.storageClient.BucketName())))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist invoice"})
		return
	}

	// Enqueue async processing task
	if err := h.queueClient.EnqueueInvoiceProcessing(ctx, inv.ID, inv.FileURL); err != nil {
		h.logger.Error("failed to enqueue processing task", zap.Error(err))
		// The invoice remains in DB; the client can retry later.
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":     inv.ID,
		"status": inv.Status,
		"url":    inv.FileURL,
	})
}

// List handles GET /api/v1/invoices
// Query params: page (default 1), limit (default 20)
func (h *InvoiceHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	uid, err := getUserID(c)
	if err != nil {
		h.logger.Warn("unauthenticated list attempt", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	invoices, total, err := h.invoiceRepo.ListByUser(ctx, uid, limit, offset)
	if err != nil {
		h.logger.Error("failed to list invoices", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve invoices"})
		return
	}

	// Build response payload
	type invoiceResp struct {
		ID        string          `json:"id"`
		FileURL   string          `json:"file_url"`
		Status    string          `json:"status"`
		CreatedAt time.Time       `json:"created_at"`
		Metadata  json.RawMessage `json:"metadata,omitempty"`
	}
	resp := make([]invoiceResp, len(invoices))
	for i, inv := range invoices {
		resp[i] = invoiceResp{
			ID:        inv.ID,
			FileURL:   inv.FileURL,
			Status:    inv.Status,
			CreatedAt: inv.CreatedAt,
			Metadata:  inv.ExtractedJSON,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"page":       page,
		"limit":      limit,
		"total":      total,
		"invoices":   resp,
	})
}

// Get handles GET /api/v1/invoices/:id
func (h *InvoiceHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()

	uid, err := getUserID(c)
	if err != nil {
		h.logger.Warn("unauthenticated get attempt", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	invID := c.Param("id")
	if strings.TrimSpace(invID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invoice id required"})
		return
	}

	inv, err := h.invoiceRepo.GetByID(ctx, invID)
	if err != nil {
		if repo.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "invoice not found"})
			return
		}
		h.logger.Error("failed to fetch invoice", zap.String("invoice_id", invID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve invoice"})
		return
	}
	if inv.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            inv.ID,
		"file_url":      inv.FileURL,
		"status":        inv.Status,
		"raw_text":      inv.RawText,
		"extracted_json": inv.ExtractedJSON,
		"created_at":    inv.CreatedAt,
		"updated_at":    inv.UpdatedAt,
	})
}

// Delete handles DELETE /api/v1/invoices/:id
func (h *InvoiceHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	uid, err := getUserID(c)
	if err != nil {
		h.logger.Warn("unauthenticated delete attempt", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	invID := c.Param("id")
	if strings.TrimSpace(invID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invoice id required"})
		return
	}

	inv, err := h.invoiceRepo.GetByID(ctx, invID)
	if err != nil {
		if repo.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "invoice not found"})
			return
		}
		h.logger.Error("failed to fetch invoice for deletion", zap.String("invoice_id", invID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete invoice"})
		return
	}
	if inv.UserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Remove object from storage (ignore error if object already missing)
	objectKey := strings.TrimPrefix(inv.FileURL, fmt.Sprintf("%s/%s/", h.storageClient.EndpointURL().String(), h.storageClient.BucketName()))
	if err := h.storageClient.RemoveObject(ctx, h.storageClient.BucketName(), objectKey); err != nil && !isNotFoundError(err) {
		h.logger.Warn("failed to delete object from storage", zap.String("object_key", objectKey), zap.Error(err))
		// Continue with DB deletion – storage may be cleaned up later.
	}

	if err := h.invoiceRepo.Delete(ctx, invID); err != nil {
		h.logger.Error("failed to delete invoice record", zap.String("invoice_id", invID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete invoice"})
		return
	}

	c.Status(http.StatusNoContent)
}

// isNotFoundError determines whether an error from MinIO indicates a missing object.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// MinIO returns an error of type *minio.ErrorResponse with Code = "NoSuchKey"
	if respErr, ok := err.(minio.ErrorResponse); ok && respErr.Code == "NoSuchKey" {
		return true
	}
	return false
}

// RegisterRoutes attaches the invoice handlers to the supplied router group.
func (h *InvoiceHandler) RegisterRoutes(rg *gin.RouterGroup) {
	invoices := rg.Group("/invoices")
	invoices.POST("", h.Upload)
	invoices.GET("", h.List)
	invoices.GET("/:id", h.Get)
	invoices.DELETE("/:id", h.Delete)
}