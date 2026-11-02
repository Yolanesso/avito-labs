package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writer := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			started := time.Now()

			next.ServeHTTP(writer, r)

			status := writer.Status()
			if status == 0 {
				status = http.StatusOK
			}

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route != "" {
				if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
					labeler.Add(attribute.String("http.route", route))
				}
				trace.SpanFromContext(r.Context()).SetAttributes(
					attribute.String("http.route", route),
				)
			} else {
				route = "unmatched"
			}

			level := slog.LevelInfo
			if status >= http.StatusInternalServerError {
				level = slog.LevelError
			}

			fields := []any{
				"method", r.Method,
				"route", route,
				"status", status,
				"duration_ms", time.Since(started).Seconds() * 1000,
			}

			span := trace.SpanFromContext(r.Context())
			span.SetName(r.Method + " " + route)
			if status >= http.StatusBadRequest {
				span.SetStatus(codes.Error, http.StatusText(status))
				span.SetAttributes(attribute.String("error.type", strconv.Itoa(status)))
			}

			spanContext := span.SpanContext()
			if spanContext.IsValid() {
				fields = append(fields,
					"trace_id", spanContext.TraceID().String(),
					"span_id", spanContext.SpanID().String(),
				)
			}

			logger.Log(r.Context(), level, "request handled", fields...)
		})
	}
}
