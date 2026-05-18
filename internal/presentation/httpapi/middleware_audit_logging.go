package httpapi

import (
	"log"
	"net/http"
	"strings"
	"time"
)

type AuditLoggingConfig struct {
	// SkipPaths avoids audit noise for endpoints that should never be audited.
	SkipPaths map[string]struct{}
}

func DefaultAuditLoggingConfig() AuditLoggingConfig {
	return AuditLoggingConfig{SkipPaths: map[string]struct{}{}}
}

// AuditLoggingMiddleware emits one JSON audit log line for mutating requests.
//
// Policy:
// - Only logs methods that can mutate state: POST/PUT/PATCH/DELETE.
// - Keeps labels low-cardinality (no query strings).
// - Never logs credentials.
//
// Recommended placement: inside AuthMiddleware so principal/scope are present.
func AuditLoggingMiddleware(cfg AuditLoggingConfig) func(http.Handler) http.Handler {
	if cfg.SkipPaths == nil {
		cfg = DefaultAuditLoggingConfig()
	}

	isAuditedMethod := func(m string) bool {
		switch m {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			return true
		default:
			return false
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := requestPathNoQuery(r)
			if _, ok := cfg.SkipPaths[path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			if !isAuditedMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			cw := newStatusCapturingResponseWriter(w)
			next.ServeHTTP(cw, r)

			entry := newRequestLogEntry(r, cw.Status(), cw.Bytes(), time.Since(start))
			entry.Type = "audit"
			entry.Action = strings.ToLower(r.Method) + " " + path
			log.Print(formatRequestLog(entry))
		})
	}
}
