package telemetry

import (
	"context"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const (
	resultSuccess   = "success"
	resultError     = "error"
	resultCancelled = "cancelled"
	resultDropped   = "dropped"
	requestsMetric  = "tripgo.position_requests"
)

func TestPositionMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	old := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	defer otel.SetMeterProvider(old)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	m, err := NewPositionMetrics()
	if err != nil {
		t.Fatal(err)
	}
	registration, err := m.ObservePool(func() (int64, int64) { return 7, 2 })
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := registration.Unregister(); err != nil {
			t.Error(err)
		}
	}()
	ctx := context.Background()
	m.Saved(ctx)
	m.RowsRead(ctx, 250)
	m.RowsRead(ctx, 0)
	for _, result := range []string{"queued", "duplicate", "dropped"} {
		m.Check(ctx, result)
	}
	for _, result := range []string{resultSuccess, resultError, resultCancelled, resultDropped} {
		m.Request(ctx, result, 100*time.Millisecond)
	}
	var data metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &data); err != nil {
		t.Fatal(err)
	}
	found := make(map[string]bool)
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			found[m.Name] = true
			switch value := m.Data.(type) {
			case metricdata.Sum[int64]:
				assertPositionCounter(t, m.Name, value)
			case metricdata.Gauge[int64]:
				want := int64(7)
				if m.Name == "tripgo.position_requests.workers.active" {
					want = 2
				}
				if len(value.DataPoints) != 1 || value.DataPoints[0].Value != want {
					t.Errorf("%s: gauge = %+v", m.Name, value)
				}
			case metricdata.Histogram[float64]:
				if len(value.DataPoints) != 2 {
					t.Errorf("duration should only include actual requests: %+v", value)
				}
				for _, point := range value.DataPoints {
					if point.Count != 1 || point.Sum != 0.1 {
						t.Errorf("wrong duration: %+v", point)
					}
					assertMetricResult(t, point.Attributes, resultSuccess, resultError)
				}
			case metricdata.Histogram[int64]:
				if len(value.DataPoints) != 1 || value.DataPoints[0].Count != 2 ||
					value.DataPoints[0].Sum != 250 {
					t.Errorf("scanned rows must be recorded once per tick: %+v", value)
				}
			default:
				t.Errorf("unexpected metric type: %T", value)
			}
		}
	}
	for _, name := range []string{
		"tripgo.positions.saved", "tripgo.stale_checks", requestsMetric,
		"tripgo.position_requests.duration", "tripgo.position_requests.queue.size",
		"tripgo.position_requests.workers.active",
		"tripgo.stale_checks.rows_read",
	} {
		if !found[name] {
			t.Errorf("missing metric %s", name)
		}
	}
}

func assertPositionCounter(t *testing.T, name string, value metricdata.Sum[int64]) {
	t.Helper()
	want := 3
	switch name {
	case "tripgo.positions.saved":
		want = 1
	case requestsMetric:
		want = 4
	}
	if len(value.DataPoints) != want {
		t.Errorf("%s: points = %d, want %d", name, len(value.DataPoints), want)
	}
	for _, point := range value.DataPoints {
		if point.Value != 1 {
			t.Errorf("%s: count = %d", name, point.Value)
		}
		if name == requestsMetric {
			assertMetricResult(t, point.Attributes, resultSuccess, resultError, resultCancelled, resultDropped)
		}
	}
}

func assertMetricResult(t *testing.T, attributes attribute.Set, allowed ...string) {
	t.Helper()
	result, ok := attributes.Value("result")
	if !ok || !slices.Contains(allowed, result.AsString()) {
		t.Errorf("metric result = %v, want one of %v", result, allowed)
	}
}
