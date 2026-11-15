package middleware

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func Metrics(router chi.Router, logger *slog.Logger) func(http.Handler) http.Handler {
	meter := otel.Meter("tripgo/http")

	active, err := meter.Int64UpDownCounter("http.server.active_requests")
	if err != nil {
		logger.Error("create active requests metric", "error", err)
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
			}

			routeContext := chi.NewRouteContext()
			if router.Match(routeContext, r.Method, r.URL.Path) {
				attrs = append(attrs,
					attribute.String("http.route", routeContext.RoutePattern()),
				)
			}

			options := metric.WithAttributes(attrs...)
			active.Add(r.Context(), 1, options)
			defer active.Add(r.Context(), -1, options)

			next.ServeHTTP(w, r)
		})
	}
}
