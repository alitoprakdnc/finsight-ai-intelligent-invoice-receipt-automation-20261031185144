package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/fin-sight/internal/logger"
)

// Logger returns a Gin middleware that logs each HTTP request in a structured
// way using Zap. The log includes HTTP method, request path, response status,
// latency, client IP and the authenticated user ID (if present).
func Logger() gin.HandlerFunc {
	// Retrieve the shared logger instance configured for the application.
	// The internal/logger package is responsible for creating a production‑ready
	// Zap logger (JSON encoder, sampling, etc.).
	log := logger.GetLogger()

	return func(c *gin.Context) {
		// Record the start time so we can calculate request latency.
		start := time.Now()

		// Continue down the chain of handlers.
		c.Next()

		// Gather data after the request has been processed.
		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		clientIP := c.ClientIP()

		// The auth middleware stores the user identifier in the context under
		// the key "userID". If it is missing we treat the request as anonymous.
		userID := "anonymous"
		if v, ok := c.Get("userID"); ok {
			userID = v
		}

		// Build the base set of fields for the log entry.
		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", method),
			zap.String("path", path),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
			zap.Any("user_id", userID),
		}

		// If any errors were added to the Gin context, log them at Error level.
		if len(c.Errors) > 0 {
			fields = append(fields, zap.Strings("errors", c.Errors.Errors()))
			log.Error("request completed with errors", fields...)
			return
		}

		// Normal successful request.
		log.Info("request completed", fields...)
	}
}