package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	ResponseBytes   *prometheus.HistogramVec
}

func NewHTTPMetrics(reg prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "alebus",
			Subsystem: "api",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests served",
		}, []string{"method", "path", "status"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "alebus",
			Subsystem: "api",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"method", "path"}),
		ResponseBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "alebus",
			Subsystem: "api",
			Name:      "http_response_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   []float64{200, 500, 1_000, 5_000, 10_000, 50_000, 200_000, 1_000_000},
		}, []string{"method", "path"}),
	}

	if reg != nil {
		reg.MustRegister(m.RequestsTotal, m.RequestDuration, m.ResponseBytes)
	}
	return m
}

// HTTPMetricsMiddleware records basic request metrics.
//
// It is safe for SSE and WebSocket handlers (preserves Hijacker/Flusher).
func HTTPMetricsMiddleware(m *HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := requestPathNoQuery(r)
			// Avoid self-observation recursion/noise.
			if path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			cw := newStatusCapturingResponseWriter(w)
			next.ServeHTTP(cw, r)

			if m == nil {
				return
			}

			status := cw.Status()
			m.RequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(status)).Inc()
			m.RequestDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
			m.ResponseBytes.WithLabelValues(r.Method, path).Observe(float64(cw.Bytes()))
		})
	}
}
