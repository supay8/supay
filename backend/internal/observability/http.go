package observability

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// HTTPMiddleware records request metrics and structured logs without reading
// bodies, query strings, authorization headers, client IPs or concrete URL IDs.
func HTTPMiddleware(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			finish := metrics.BeginHTTPRequest()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				route := chi.RouteContext(r.Context()).RoutePattern()
				if route == "" {
					route = "unmatched"
				}
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}
				duration := time.Since(started)
				finish(r.Method, route, status, duration)
				attrs := []any{
					"request_id", middleware.GetReqID(r.Context()),
					"method", r.Method,
					"route", route,
					"status", status,
					"duration_ms", duration.Milliseconds(),
				}
				if status >= http.StatusInternalServerError {
					slog.Error("http request completed", attrs...)
				} else {
					slog.Info("http request completed", attrs...)
				}
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
