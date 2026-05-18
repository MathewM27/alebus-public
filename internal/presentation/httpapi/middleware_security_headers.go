package httpapi

import "net/http"

// SecurityHeadersMiddleware sets baseline security headers.
//
// - HSTS is only set when the request is HTTPS (TLS or trusted proxy proto=https)
// - Other headers are safe on both HTTP and HTTPS.
func SecurityHeadersMiddleware(enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if enabled {
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("X-Frame-Options", "DENY")
				w.Header().Set("Referrer-Policy", "no-referrer")

				proto := ClientProtoFromContext(r.Context())
				if proto == "" {
					proto = "http"
					if r.TLS != nil {
						proto = "https"
					}
				}
				if proto == "https" {
					w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
