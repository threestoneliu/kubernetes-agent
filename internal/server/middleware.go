package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"
const requestIDCtxKey = "request_id"

// requestIDMiddleware ensures every request has a request id. If the
// client sent one (X-Request-ID), it's reused; otherwise a new UUID
// is minted. The id is echoed in the response header so callers can
// correlate logs.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := contextWithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loggerMiddleware emits a single slog line per request, after the
// handler returns so the status code is final. Format mirrors the
// common access-log shape: method, path, status, duration, request id.
//
// Embedded static assets (the SPA's hashed JS/CSS chunks and
// index.html) are skipped — logging every .js chunk on page load
// drowns out the API traffic we actually want to see.
func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestIDFromContext(r.Context()),
		)
	})
}

// recovererMiddleware catches a panic in any downstream handler and
// returns a 500 with the standard error envelope. The panic is
// logged with the full stack so the operator can find the bug; the
// client gets a generic message + retryable=true (internal errors
// are usually transient — e.g. database hiccup).
func recovererMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic in handler",
					"panic", rec,
					"stack", string(debug.Stack()),
					"request_id", requestIDFromContext(r.Context()),
				)
				writeError(w, http.StatusInternalServerError, "internal",
					"an internal error occurred", true)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware sets a permissive CORS envelope. The MVP runs as a
// single binary alongside the web UI on a different port during
// development; tighten this for production.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, Last-Event-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code
// after the handler returns. It defaults to 200 (the standard
// implicit default) when the handler never calls WriteHeader.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush is forwarded so SSE handlers can still flush through the
// wrapped writer. chi's logger sits between the handler and the
// real connection — without this, Flush() on the real writer is
// unreachable from inside the handler.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
