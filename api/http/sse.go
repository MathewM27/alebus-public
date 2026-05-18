package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type SSEConnectionLimiter struct {
	max    int64
	active int64
}

func NewSSEConnectionLimiter(max int64) *SSEConnectionLimiter {
	return &SSEConnectionLimiter{max: max}
}

func (l *SSEConnectionLimiter) Acquire() bool {
	if l == nil {
		return true
	}
	if l.max <= 0 {
		atomic.AddInt64(&l.active, 1)
		return true
	}
	for {
		cur := atomic.LoadInt64(&l.active)
		if cur >= l.max {
			return false
		}
		if atomic.CompareAndSwapInt64(&l.active, cur, cur+1) {
			return true
		}
	}
}

func (l *SSEConnectionLimiter) Release() {
	if l == nil {
		return
	}
	atomic.AddInt64(&l.active, -1)
}

func (l *SSEConnectionLimiter) Active() int64 {
	if l == nil {
		return 0
	}
	return atomic.LoadInt64(&l.active)
}

type sseWriter struct {
	w        http.ResponseWriter
	flusher  http.Flusher
	ctrl     *http.ResponseController
	writeTTL time.Duration
}

func newSSEWriter(w http.ResponseWriter, writeTTL time.Duration) (*sseWriter, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	ctrl := http.NewResponseController(w)
	return &sseWriter{w: w, flusher: f, ctrl: ctrl, writeTTL: writeTTL}, nil
}

func (s *sseWriter) setHeaders() {
	h := s.w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Best-effort: disable proxy buffering (nginx)
	h.Set("X-Accel-Buffering", "no")
}

func (s *sseWriter) writeEvent(event string, data any) error {
	if s.writeTTL > 0 {
		_ = s.ctrl.SetWriteDeadline(time.Now().Add(s.writeTTL))
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.w, "event: %s\n", event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.w, "data: %s\n\n", payload)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func parseCSVParam(v string) []string {
	if v == "" {
		return nil
	}
	out := make([]string, 0)
	start := 0
	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == ',' {
			part := v[start:i]
			start = i + 1
			for len(part) > 0 && part[0] == ' ' {
				part = part[1:]
			}
			for len(part) > 0 && part[len(part)-1] == ' ' {
				part = part[:len(part)-1]
			}
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func mustContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
