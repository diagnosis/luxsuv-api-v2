package logger

import (
	"net/http"
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/helper"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

func redactIP(ip string) string {
	if ip == "" {
		return "unknown"
	}
	return ip
}

func redactUserAgent(ua string) string {
	if len(ua) > 100 {
		return ua[:100] + "..."
	}
	return ua
}

func HandlerLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		correlatedID := r.Header.Get("X-Correlation-ID")
		if correlatedID == "" {
			correlatedID = helper.GenerateID()
		}
		ctx := WithCorrelationID(r.Context(), correlatedID)
		r = r.WithContext(ctx)
		w.Header().Set("X-Correlation-ID", correlatedID)
		wrapped := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(wrapped, r)
		duration := time.Since(start)

		Info(ctx, "request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration_ms", duration.Milliseconds(),
			"size_bytes", wrapped.size,
			"ip", redactIP(helper.ClientIP(r)),
			"user_agent", redactUserAgent(r.UserAgent()),
		)

	})
}
