package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Recovery is a Gin middleware that recovers from any panics,
// logs the error with a unique identifier and stack trace,
// and returns a generic 500 response containing the error ID.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				// Generate a unique error identifier for correlation.
				errID := uuid.NewString()

				// Capture the stack trace.
				stack := string(debug.Stack())

				// Log the panic with structured fields.
				zap.L().Error("panic recovered",
					zap.Any("panic", rec),
					zap.String("error_id", errID),
					zap.String("stack_trace", stack),
				)

				// Abort the request and respond with a generic error payload.
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":    "internal server error",
					"error_id": errID,
				})
			}
		}()

		// Continue down the chain to the next handler.
		c.Next()
	}
}