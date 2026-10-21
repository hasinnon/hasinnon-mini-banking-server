package httpapi

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type middleware func(http.Handler) http.Handler

func Chain(h http.Handler, m ...middleware) http.Handler {
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}

type ctxKey string

const requestIDKey ctxKey = "request_id"

func RequestID() middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get("X-Request-ID")
			if strings.TrimSpace(rid) == "" {
				rid = uuid.NewString()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, rid)
			w.Header().Set("X-Request-ID", rid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetRequestID(ctx context.Context) string {
	v := ctx.Value(requestIDKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func AccessLog() middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(sw, r)
			log.Printf("rid=%s ip=%s method=%s path=%s status=%d latency_ms=%d",
				GetRequestID(r.Context()),
				r.RemoteAddr,
				r.Method,
				r.URL.Path,
				sw.status,
				time.Since(start).Milliseconds(),
			)
		})
	}
}

func MaxConcurrent(n int) middleware {
	sem := make(chan struct{}, n)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			default:
				writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many concurrent clients"})
			}
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func Health(mode string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mode": mode})
	}
}