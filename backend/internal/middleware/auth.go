package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"

	"backend/internal/model"
	"backend/internal/repo"
)

type contextKey string

const userContextKey contextKey = "authUser"

// AuthMiddleware returns a Gin middleware that validates JWT tokens,
// loads the corresponding user (including roles) from the repository
// and stores the user in the request context.
//
// The JWT is expected to be signed with HMAC using the provided secret.
// The token must contain a `sub` claim holding the user ID as a string.
func AuthMiddleware(userRepo repo.UserRepository, jwtSecret string) gin.HandlerFunc {
	secretBytes := []byte(jwtSecret)

	return func(c *gin.Context) {
		// --------------------------------------------------------------------
		// Extract and validate the Authorization header.
		// --------------------------------------------------------------------
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortUnauthorized(c, "authorization header missing")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			abortUnauthorized(c, "invalid authorization header format")
			return
		}
		tokenStr := parts[1]

		// --------------------------------------------------------------------
		// Parse and verify the JWT.
		// --------------------------------------------------------------------
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			// Enforce HMAC signing method.
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return secretBytes, nil
		})
		if err != nil || !token.Valid {
			abortUnauthorized(c, "invalid token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			abortUnauthorized(c, "invalid token claims")
			return
		}

		// --------------------------------------------------------------------
		// Extract the user identifier from the `sub` claim.
		// --------------------------------------------------------------------
		sub, ok := claims["sub"].(string)
		if !ok || sub == "" {
			abortUnauthorized(c, "subject claim missing")
			return
		}
		userID, err := strconv.ParseInt(sub, 10, 64)
		if err != nil {
			abortUnauthorized(c, "invalid subject claim")
			return
		}

		// --------------------------------------------------------------------
		// Load the user (and roles) from the repository.
		// --------------------------------------------------------------------
		user, err := userRepo.GetByID(c.Request.Context(), userID)
		if err != nil {
			// Treat any repository error as unauthorized to avoid leaking info.
			abortUnauthorized(c, "user not found")
			return
		}
		if user == nil {
			abortUnauthorized(c, "user not found")
			return
		}

		// --------------------------------------------------------------------
		// Store the user in the Gin context for downstream handlers.
		// --------------------------------------------------------------------
		c.Set(string(userContextKey), user)

		c.Next()
	}
}

// abortUnauthorized aborts the request with a 401 status and a JSON error payload.
func abortUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": message,
	})
}

// UserFromContext retrieves the authenticated user from the Gin context.
// The boolean return value indicates whether a user was present.
func UserFromContext(c *gin.Context) (*model.User, bool) {
	val, exists := c.Get(string(userContextKey))
	if !exists {
		return nil, false
	}
	user, ok := val.(*model.User)
	return user, ok
}

// MustGetUser extracts the authenticated user from the context.
// It panics if the user is not present; useful in handlers that are
// guaranteed to be behind the AuthMiddleware.
func MustGetUser(c *gin.Context) *model.User {
	user, ok := UserFromContext(c)
	if !ok || user == nil {
		panic("authenticated user not found in context")
	}
	return user
}