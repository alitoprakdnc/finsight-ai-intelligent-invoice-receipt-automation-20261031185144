package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"backend/internal/config"
	"backend/internal/handler"
	"backend/internal/middleware"
)

// NewRouter creates and configures the Gin router for the FinSight AI backend.
// It registers all API groups, attaches global middleware, and sets up
// Swagger UI and a JSON 404 handler.
func NewRouter(cfg *config.Config) *gin.Engine {
	// Use Gin in release mode when not debugging.
	if cfg.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create a new router without any default middleware.
	router := gin.New()

	// Global middleware.
	router.Use(
		middleware.Logger(),
		middleware.Recovery(),
	)

	// -------------------------------------------------------------------------
	// Health endpoint – no authentication required.
	// -------------------------------------------------------------------------
	router.GET("/api/health", handler.Health)

	// -------------------------------------------------------------------------
	// Swagger UI – serves OpenAPI spec.
	// -------------------------------------------------------------------------
	router.GET("/docs/*any", ginSwagger.WrapHandler(files.Handler))

	// -------------------------------------------------------------------------
	// Authentication routes (login, token refresh, etc.).
	// These handlers are expected to be defined in the auth package.
	// -------------------------------------------------------------------------
	authGroup := router.Group("/api/auth")
	{
		authGroup.POST("/login", handler.Login)           // #nosec G101 – handled in handler
		authGroup.POST("/refresh", handler.RefreshToken)   // token refresh
		authGroup.POST("/logout", handler.Logout)         // optional logout
	}

	// -------------------------------------------------------------------------
	// Invoice routes – protected by JWT authentication.
	// -------------------------------------------------------------------------
	invoiceGroup := router.Group("/api/invoices")
	invoiceGroup.Use(middleware.AuthRequired())
	{
		invoiceGroup.POST("/", handler.UploadInvoice)               // multipart upload
		invoiceGroup.GET("/", handler.ListInvoices)                 // pagination support
		invoiceGroup.GET("/:id", handler.GetInvoice)                // detail view
		invoiceGroup.DELETE("/:id", handler.DeleteInvoice)          // soft‑delete
		invoiceGroup.PATCH("/:id", handler.UpdateInvoice)          // optional manual correction endpoint
	}

	// -------------------------------------------------------------------------
	// Webhook subscription routes – also protected.
	// -------------------------------------------------------------------------
	webhookGroup := router.Group("/api/webhooks")
	webhookGroup.Use(middleware.AuthRequired())
	{
		webhookGroup.POST("/", handler.CreateWebhook)
		webhookGroup.GET("/", handler.ListWebhooks)
		webhookGroup.DELETE("/:id", handler.DeleteWebhook)
	}

	// -------------------------------------------------------------------------
	// Fallback for undefined routes – return a clean JSON 404.
	// -------------------------------------------------------------------------
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "endpoint not found",
			"path":    c.Request.URL.Path,
			"method":  c.Request.Method,
			"message": "the requested resource does not exist",
		})
	})

	return router
}