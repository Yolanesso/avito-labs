package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type PositionMetrics struct {
	saved    metric.Int64Counter
	checks   metric.Int64Counter
	requests metric.Int64Counter
	duration metric.Float64Histogram
	rows     metric.Int64Histogram
}

func NewPositionMetrics() (*PositionMetrics, error) {
	meter := otel.Meter("tripgo")
	m := &PositionMetrics{}
	var err error
	m.saved, err = meter.Int64Counter("tripgo.positions.saved")
	if err != nil {
		return nil, fmt.Errorf("create positions counter: %w", err)
	}
	m.checks, err = meter.Int64Counter("tripgo.stale_checks")
	if err != nil {
		return nil, fmt.Errorf("create stale checks counter: %w", err)
	}
	m.requests, err = meter.Int64Counter("tripgo.position_requests")
	if err != nil {
		return nil, fmt.Errorf("create requests counter: %w", err)
	}
	m.duration, err = meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("create requests duration: %w", err)
	}
	m.rows, err = meter.Int64Histogram("tripgo.stale_checks.rows_read",
		metric.WithDescription("Rows read from PostgreSQL per scanner tick"),
		metric.WithExplicitBucketBoundaries(0, 10, 100, 250, 1000, 10000))
	if err != nil {
		return nil, fmt.Errorf("create scanned rows histogram: %w", err)
	}
	return m, nil
}

func (m *PositionMetrics) RowsRead(ctx context.Context, rows int) {
	m.rows.Record(ctx, int64(rows))
}

// ObservePool возвращает регистрацию, которую нужно снять после остановки пула.
func (m *PositionMetrics) ObservePool(stats func() (int64, int64)) (metric.Registration, error) {
	meter := otel.Meter("tripgo")
	queue, err := meter.Int64ObservableGauge("tripgo.position_requests.queue.size")
	if err != nil {
		return nil, fmt.Errorf("create queue gauge: %w", err)
	}
	workers, err := meter.Int64ObservableGauge("tripgo.position_requests.workers.active")
	if err != nil {
		return nil, fmt.Errorf("create workers gauge: %w", err)
	}
	registration, err := meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		queued, active := stats()
		observer.ObserveInt64(queue, queued)
		observer.ObserveInt64(workers, active)
		return nil
	}, queue, workers)
	if err != nil {
		return nil, fmt.Errorf("register position gauges: %w", err)
	}
	return registration, nil
}

func (m *PositionMetrics) Saved(ctx context.Context) {
	m.saved.Add(ctx, 1)
}

func (m *PositionMetrics) Check(ctx context.Context, result string) {
	m.checks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (m *PositionMetrics) Request(ctx context.Context, result string, duration time.Duration) {
	options := metric.WithAttributes(attribute.String("result", result))
	m.requests.Add(ctx, 1, options)
	if result == "success" || result == "error" {
		m.duration.Record(ctx, duration.Seconds(), options)
	}
}
