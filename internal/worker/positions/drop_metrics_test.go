package positions

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"
)

func TestOverflowAndEvictionIncrementBothDroppedCounters(t *testing.T) {
	for _, evict := range []bool{false, true} {
		t.Run(map[bool]string{false: "drop incoming", true: "evict newest"}[evict], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			metrics := NewMockmetrics(ctrl)
			p := New(Config{QueueSize: 1}, nil, nil, metrics, slog.Default(), nil)
			first, incoming := staleTrip(), staleTrip()
			if evict {
				incoming.PositionAt = first.PositionAt.Add(-time.Second)
				metrics.EXPECT().Check(gomock.Any(), resultQueued)
			}
			p.enqueue(first, trace.SpanContext{})
			metrics.EXPECT().Check(gomock.Any(), resultDropped).Times(1)
			metrics.EXPECT().Request(gomock.Any(), resultDropped, time.Duration(0)).Times(1)
			if !p.admit(context.Background(), incoming) {
				t.Fatal("overflow must stop the scanner")
			}
		})
	}
}
