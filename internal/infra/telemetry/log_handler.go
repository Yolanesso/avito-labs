package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// Даже у стартовых сообщений без спана поля trace_id и span_id присутствуют.
type traceHandler struct {
	slog.Handler
}

func (h traceHandler) Handle(ctx context.Context, record slog.Record) error {
	hasTrace, hasSpan := false, false
	record.Attrs(func(a slog.Attr) bool {
		hasTrace = hasTrace || a.Key == "trace_id"
		hasSpan = hasSpan || a.Key == "span_id"
		return true
	})
	sc := trace.SpanContextFromContext(ctx)
	traceID, spanID := "", ""
	if sc.IsValid() {
		traceID, spanID = sc.TraceID().String(), sc.SpanID().String()
	}
	if !hasTrace {
		record.AddAttrs(slog.String("trace_id", traceID))
	}
	if !hasSpan {
		record.AddAttrs(slog.String("span_id", spanID))
	}
	return h.Handler.Handle(ctx, record)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}
