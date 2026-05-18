package httpapi

import (
	"net/http"

	apihttp "github.com/MathewM27/busTrack-alebus/api/http"
)

// MaxBodyBytesMiddleware limits request body size.
//
// It rejects requests with a known Content-Length above the limit using 413,
// and wraps the body with http.MaxBytesReader for additional protection.
func MaxBodyBytesMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if maxBytes <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				apihttp.WriteError(
					w,
					http.StatusRequestEntityTooLarge,
					"payload_too_large",
					"Request body too large",
					map[string]any{"maxBytes": maxBytes, "contentLength": r.ContentLength},
				)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
