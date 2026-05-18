package httpapi

import (
	"log"
	"net/http"
	"time"
)

type RequestLoggingConfig struct {
	// SkipPaths avoids noisy logs for very hot endpoints like /metrics.
	SkipPaths map[string]struct{}
}

func DefaultRequestLoggingConfig() RequestLoggingConfig {
	return RequestLoggingConfig{SkipPaths: map[string]struct{}{}}
}

// RequestLoggingMiddleware logs one JSON line per request.
//
// IMPORTANT: This middleware should be placed *inside* AuthMiddleware if you
// want scope/principal data on successful requests.
// Auth denials should be logged by AuthMiddleware itself.
func RequestLoggingMiddleware(cfg RequestLoggingConfig) func(http.Handler) http.Handler {
	if cfg.SkipPaths == nil {
		cfg = DefaultRequestLoggingConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := cfg.SkipPaths[requestPathNoQuery(r)]; ok {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			cw := newStatusCapturingResponseWriter(w)
			next.ServeHTTP(cw, r)

			entry := newRequestLogEntry(r, cw.Status(), cw.Bytes(), time.Since(start))
			log.Print(formatRequestLog(entry))
		})
	}
}
