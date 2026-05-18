package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	apihttp "github.com/MathewM27/busTrack-alebus/api/http"
)

// RequestLogEntry is a structured log line for a single HTTP request.
//
// Policy:
// - Never log credentials (Authorization, X-API-Key, X-Dev-Secret)
// - Keep labels low-cardinality (no query strings)
// - Prefer stable fields that help correlate incidents
//
// This is intentionally stdlib-only (log.Print in callers) to keep dependencies minimal.
type RequestLogEntry struct {
	Type       string `json:"type"`
	Timestamp  string `json:"ts"`
	RequestID  string `json:"request_id,omitempty"`
	ClientIP   string `json:"client_ip,omitempty"`
	Proto      string `json:"proto,omitempty"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	Bytes      int64  `json:"bytes"`
	DurationMs int64  `json:"duration_ms"`

	// Action is optional and primarily used for audit log entries.
	Action string `json:"action,omitempty"`

	Scope      string `json:"scope,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	OperatorID string `json:"operator_id,omitempty"`

	Error string `json:"error,omitempty"`
}

func formatRequestLog(e RequestLogEntry) string {
	b, err := json.Marshal(e)
	if err != nil {
		// Last-ditch fallback: keep it machine-parsable.
		return `{"type":"request","ts":"` + time.Now().UTC().Format(time.RFC3339) + `","error":"json_marshal_failed"}`
	}
	return string(b)
}

func requestPathNoQuery(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	// Avoid unbounded cardinality from query strings.
	return strings.TrimSpace(r.URL.Path)
}

func scopeString(ctxScope Scope) string {
	if ctxScope == "" {
		return ""
	}
	return string(ctxScope)
}

// newRequestLogEntry builds a RequestLogEntry using context-derived metadata when available.
func newRequestLogEntry(r *http.Request, status int, bytes int64, duration time.Duration) RequestLogEntry {
	ctx := r.Context()

	reqID := GetRequestIDFromContext(ctx)
	clientIP := ClientIPFromContext(ctx)
	proto := ClientProtoFromContext(ctx)

	p, _ := apihttp.PrincipalFromContext(ctx)
	scope := ScopeFromContext(ctx)

	return RequestLogEntry{
		Type:       "request",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		RequestID:  reqID,
		ClientIP:   clientIP,
		Proto:      proto,
		Method:     r.Method,
		Path:       requestPathNoQuery(r),
		Status:     status,
		Bytes:      bytes,
		DurationMs: duration.Milliseconds(),
		Scope:      scopeString(scope),
		UserID:     strings.TrimSpace(p.UserID),
		OperatorID: strings.TrimSpace(p.OperatorID),
	}
}
