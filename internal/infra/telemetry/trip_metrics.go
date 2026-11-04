package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type TripMetrics struct {
	created   metric.Int64Counter
	completed metric.Int64Counter
	logger    *slog.Logger
}

func NewTripMetrics(logger *slog.Logger) (*TripMetrics, error) {
	meter := otel.Meter("tripgo")

	created, err := meter.Int64Counter("tripgo.trips.created")
	if err != nil {
		return nil, fmt.Errorf("create trips created counter: %w", err)
	}

	completed, err := meter.Int64Counter("tripgo.trips.completed")
	if err != nil {
		return nil, fmt.Errorf("create trips completed counter: %w", err)
	}

	return &TripMetrics{
		created:   created,
		completed: completed,
		logger:    logger,
	}, nil
}

func (m *TripMetrics) Created(ctx context.Context, tripID uuid.UUID) {
	m.created.Add(ctx, 1)
	m.logEvent(ctx, "trip created", tripID)
}

func (m *TripMetrics) Completed(ctx context.Context, tripID uuid.UUID) {
	m.completed.Add(ctx, 1)
	m.logEvent(ctx, "trip completed", tripID)
}

func (m *TripMetrics) logEvent(ctx context.Context, message string, tripID uuid.UUID) {
	fields := []any{"trip_id", tripID.String()}
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		fields = append(fields,
			"trace_id", spanContext.TraceID().String(),
			"span_id", spanContext.SpanID().String(),
		)
	}
	m.logger.InfoContext(ctx, message, fields...)
}
