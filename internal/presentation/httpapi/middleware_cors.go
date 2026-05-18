package httpapi

import (
	"net/http"
	"strings"
)

// CORSMiddleware adds minimal CORS support for browser REST calls.
//
// Design goals:
// - Strict allowlist (exact origin matches)
// - Adds headers to both success and error responses (wrap outside auth)
// - Handles preflight OPTIONS requests
// - Skips WS/SSE endpoints (they have their own browser constraints)
//
// If allowedOrigins is empty, this middleware is a no-op.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		allowed[o] = struct{}{}
	}

	isAllowed := func(origin string) bool {
		if origin == "" {
			return false
		}
		_, ok := allowed[origin]
		return ok
	}

	shouldSkipPath := func(path string) bool {
		// WebSockets
		if strings.HasPrefix(path, "/api/v1/ws/") {
			return true
		}
		// Streaming SSE
		if strings.HasPrefix(path, "/api/v1/stream/") {
			return true
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowed) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			if shouldSkipPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				// Non-browser or same-origin; nothing to add.
				next.ServeHTTP(w, r)
				return
			}

			if !isAllowed(origin) {
				// Don't add any CORS headers for untrusted origins.
				// For preflight, fail fast.
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Allowed origin: add headers.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

			// Preflight
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

				// Allow a minimal set of request headers. If the browser requests others,
				// we reject to avoid accidentally allowing arbitrary headers.
				reqHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers"))
				if reqHeaders != "" {
					allowedHeaders := map[string]struct{}{
						"authorization": {},
						"content-type":  {},
						"x-api-key":     {},
						"x-request-id":  {},
					}
					for _, h := range strings.Split(reqHeaders, ",") {
						h = strings.ToLower(strings.TrimSpace(h))
						if h == "" {
							continue
						}
						if _, ok := allowedHeaders[h]; !ok {
							w.WriteHeader(http.StatusForbidden)
							return
						}
					}
					w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
				} else {
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key, X-Request-ID")
				}

				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
