package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/mock/gomock"
)

func TestActiveTripsGaugeReadsDatabaseOnEveryCollection(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	old := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(old)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	counter := NewMockactiveTripCounter(gomock.NewController(t))
	metrics, err := NewTripMetrics(slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	registration, err := metrics.ObserveActive(counter)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := registration.Unregister(); err != nil {
			t.Error(err)
		}
	})
	for _, want := range []int64{200, 3, 0} {
		counter.EXPECT().CountActive(gomock.Any()).Return(want, nil)
		var data metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &data); err != nil {
			t.Fatal(err)
		}
		assertActiveGauge(t, data, want)
	}
	failure := errors.New("database unavailable")
	counter.EXPECT().CountActive(gomock.Any()).Return(int64(0), failure)
	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); !errors.Is(err, failure) {
		t.Fatalf("collection must report database failure, got %v", err)
	}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == "tripgo.trips.active" {
				t.Fatal("failed database query must not emit a misleading zero")
			}
		}
	}
}

func assertActiveGauge(t *testing.T, data metricdata.ResourceMetrics, want int64) {
	t.Helper()
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "tripgo.trips.active" {
				continue
			}
			value, ok := m.Data.(metricdata.Gauge[int64])
			if !ok || len(value.DataPoints) != 1 || value.DataPoints[0].Value != want {
				t.Fatalf("active gauge = %+v, want %d", m.Data, want)
			}
			return
		}
	}
	t.Fatal("active trips gauge was not collected")
}
