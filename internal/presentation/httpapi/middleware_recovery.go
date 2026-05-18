package httpapi

import (
	"log"
	"net/http"
	"runtime/debug"

	apihttp "github.com/MathewM27/busTrack-alebus/api/http"
)

// RecoveryMiddleware returns middleware that catches panics and converts them to 500 JSON errors.
//
// This middleware:
// - Catches panics in handlers
// - Logs the panic with stack trace for debugging
// - Returns a 500 JSON error envelope to the client
// - Does NOT restart the service (let orchestrator handle)
//
// Place this middleware at the TOP of the middleware stack (before all others).
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log panic with stack trace for debugging
				log.Printf(
					"❌ PANIC in handler [%s %s]: %v\nStack:\n%s",
					r.Method,
					r.RequestURI,
					err,
					string(debug.Stack()),
				)

				// Return 500 JSON error (do NOT leak stack trace to client)
				apihttp.WriteError(w, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
