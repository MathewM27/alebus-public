package httpapi

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// RequestIDContextKey is the context key for request IDs.
type contextKeyType string

const RequestIDContextKey contextKeyType = "request_id"

// RequestIDMiddleware returns middleware that generates a unique request ID for each request,
// adds it to the response header, and propagates it via context.
//
// If the client provides X-Request-ID header, that value is used.
// Otherwise, a new UUID is generated.
//
// This middleware should run early in the stack so request ID is available to all downstream handlers.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for client-provided request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			// Generate new UUID for this request
			requestID = uuid.New().String()
		}

		// Add request ID to response header
		w.Header().Set("X-Request-ID", requestID)

		// Add request ID to context for use by handlers, middleware, and logger
		ctx := context.WithValue(r.Context(), RequestIDContextKey, requestID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// GetRequestIDFromContext retrieves the request ID from the context.
// Returns empty string if not found.
func GetRequestIDFromContext(ctx context.Context) string {
	id, ok := ctx.Value(RequestIDContextKey).(string)
	if !ok {
		return ""
	}
	return id
}
