package positions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

func cursorAfter(t trip.StaleTrip) *trip.StaleTripCursor {
	return &trip.StaleTripCursor{PositionAt: t.PositionAt, ID: t.ID}
}

func TestScannerReads250TripsInThreeBatches(t *testing.T) {
	f := newFixture(t, 1, 300)
	trips := make([]trip.StaleTrip, 250)
	for i := range trips {
		trips[i] = staleTrip()
	}
	gomock.InOrder(
		f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).Return(trips[:100], nil),
		f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, cursorAfter(trips[99])).Return(trips[100:200], nil),
		f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, cursorAfter(trips[199])).Return(trips[200:], nil),
	)
	processed := make(chan uuid.UUID, len(trips))
	f.gateway.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, id uuid.UUID) error {
			processed <- id
			return nil
		}).Times(len(trips))
	// До запуска воркеров очередь достаточно велика для всех трёх батчей.
	f.pool.check(context.Background())
	if got := receive(t, f.rows); got != len(trips) {
		t.Fatalf("rows read = %d, want 250", got)
	}
	for range trips {
		assertResult(t, f.checks, "queued")
	}
	f.start(t, context.Background())
	for _, item := range trips {
		if got := receive(t, processed); got != item.ID {
			t.Fatalf("processed %s, want %s", got, item.ID)
		}
	}
}

func TestScannerStopsAtFullQueueAndRestartsFromBeginning(t *testing.T) {
	f := newFixture(t, 1, 2)
	f.pool.cfg.BatchSize = 2
	trips := []trip.StaleTrip{staleTrip(), staleTrip()}
	// Ни продолжения, ни старого курсора на следующем тике.
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 2, nil).Return(trips, nil).Times(2)
	f.pool.check(context.Background())
	f.pool.check(context.Background())
	for range 2 {
		if got := receive(t, f.rows); got != 2 {
			t.Fatalf("rows read = %d, want 2", got)
		}
	}
	for _, want := range []string{resultQueued, resultQueued, resultDuplicate, resultDuplicate} {
		assertResult(t, f.checks, want)
	}
	for _, item := range trips {
		f.gateway.EXPECT().RequestPosition(gomock.Any(), item.DriverID, item.ID).Return(nil)
	}
	f.start(t, context.Background())
}

func TestScannerStopsOnFirstDropAndRestartsFromBeginning(t *testing.T) {
	f := newFixture(t, 1, 50)
	trips := make([]trip.StaleTrip, 250)
	for i := range trips {
		trips[i] = staleTrip()
	}
	// На обоих тиках читается только первая порция, без продолжения по курсору.
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).
		Return(trips[:100], nil).Times(2)
	f.pool.check(context.Background())
	for range 50 {
		assertResult(t, f.checks, resultQueued)
	}
	assertResult(t, f.checks, resultDropped)
	assertResult(t, f.requests, resultDropped)
	if len(f.checks) != 0 {
		t.Fatal("scanner continued admitting trips after the first dropped task")
	}
	f.pool.check(context.Background())
	for range 50 {
		assertResult(t, f.checks, resultDuplicate)
	}
	assertResult(t, f.checks, resultDropped)
	assertResult(t, f.requests, resultDropped)
	if len(f.checks) != 0 || len(f.requests) != 0 {
		t.Fatal("scanner processed extra tasks or counted a dropped task more than once")
	}
	for range 2 {
		if got := receive(t, f.rows); got != 100 {
			t.Fatalf("rows read = %d, want the entire fetched batch of 100", got)
		}
	}
	for _, item := range trips[:50] {
		f.gateway.EXPECT().RequestPosition(gomock.Any(), item.DriverID, item.ID).Return(nil)
	}
	f.start(t, context.Background())
}

func TestScannerStopsOnDropEvenIfQueueBecomesEmpty(t *testing.T) {
	f := newFixture(t, 1, 1)
	f.pool.cfg.BatchSize = 3
	trips := []trip.StaleTrip{staleTrip(), staleTrip(), staleTrip()}
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 3, nil).Return(trips, nil)
	metrics := NewMockmetrics(gomock.NewController(t))
	gomock.InOrder(
		metrics.EXPECT().Check(gomock.Any(), resultQueued),
		metrics.EXPECT().Check(gomock.Any(), resultDropped).Do(func(ctx context.Context, _ string) {
			// Освобождение очереди в момент дропа не должно возобновлять обход.
			f.pool.Cancel(ctx, trips[0].ID)
		}),
	)
	metrics.EXPECT().Request(gomock.Any(), "cancelled", time.Duration(0))
	metrics.EXPECT().Request(gomock.Any(), resultDropped, time.Duration(0))
	metrics.EXPECT().RowsRead(gomock.Any(), 3)
	f.pool.metrics = metrics
	f.pool.check(context.Background())
	f.start(t, context.Background())
}

func TestScannerStopsOnEviction(t *testing.T) {
	f := newFixture(t, 1, 1)
	oldest, middle, newest := staleTrip(), staleTrip(), staleTrip()
	f.pool.enqueue(newest, trace.SpanContext{})
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).
		Return([]trip.StaleTrip{oldest, middle}, nil)
	f.pool.check(context.Background())
	assertResult(t, f.checks, resultQueued)
	assertResult(t, f.checks, resultDropped)
	assertResult(t, f.requests, resultDropped)
	if len(f.checks) != 0 || len(f.requests) != 0 {
		t.Fatal("scanner continued after eviction or counted it more than once")
	}
	f.gateway.EXPECT().RequestPosition(gomock.Any(), oldest.DriverID, oldest.ID).Return(nil)
	f.start(t, context.Background())
}

func TestScannerRecordsRowsReadBeforeBatchError(t *testing.T) {
	f := newFixture(t, 1, 10)
	f.pool.cfg.BatchSize = 2
	trips := []trip.StaleTrip{staleTrip(), staleTrip()}
	gomock.InOrder(
		f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 2, nil).Return(trips, nil),
		f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 2, cursorAfter(trips[1])).
			Return(nil, errors.New("database unavailable")),
	)
	f.pool.check(context.Background())
	if got := receive(t, f.rows); got != 2 {
		t.Fatalf("rows read before error = %d, want 2", got)
	}
	for _, item := range trips {
		f.gateway.EXPECT().RequestPosition(gomock.Any(), item.DriverID, item.ID).Return(nil)
	}
	f.start(t, context.Background())
}

func TestScannerRecordsEmptyTick(t *testing.T) {
	f := newFixture(t, 1, 2)
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).Return(nil, nil)
	f.pool.check(context.Background())
	if got := receive(t, f.rows); got != 0 {
		t.Fatalf("empty tick read = %d, want 0", got)
	}
	f.start(t, context.Background())
}
