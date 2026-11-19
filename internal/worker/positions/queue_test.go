package positions

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

func TestPriorityQueueProcessesOldestFirst(t *testing.T) {
	f := newFixture(t, 1, 3)
	oldest, middle, newest := staleTrip(), staleTrip(), staleTrip()
	for _, item := range []trip.StaleTrip{newest, oldest, middle} {
		if result, _ := f.pool.enqueue(item, trace.SpanContext{}); result != resultQueued {
			t.Fatal(result)
		}
	}
	gomock.InOrder(
		f.gateway.EXPECT().RequestPosition(gomock.Any(), oldest.DriverID, oldest.ID).Return(nil),
		f.gateway.EXPECT().RequestPosition(gomock.Any(), middle.DriverID, middle.ID).Return(nil),
		f.gateway.EXPECT().RequestPosition(gomock.Any(), newest.DriverID, newest.ID).Return(nil),
	)
	f.start(t, context.Background())
}

func TestPriorityQueueEvictsNewestAndPreservesCancellation(t *testing.T) {
	f := newFixture(t, 1, 2)
	oldest, middle, newest := staleTrip(), staleTrip(), staleTrip()
	f.pool.enqueue(newest, trace.SpanContext{})
	f.pool.enqueue(middle, trace.SpanContext{})
	f.pool.admit(context.Background(), oldest)
	assertResult(t, f.checks, "queued")
	assertResult(t, f.checks, "dropped")
	assertResult(t, f.requests, "dropped")
	// Вытесненная задача больше не считается pending.
	if result, _ := f.pool.enqueue(newest, trace.SpanContext{}); result != resultDropped {
		t.Fatalf("evicted trip re-enqueue = %q, want dropped, not duplicate", result)
	}
	if result, _ := f.pool.enqueue(oldest, trace.SpanContext{}); result != resultDuplicate {
		t.Fatal(result)
	}
	f.pool.Cancel(context.Background(), oldest.ID)
	if queued, _ := f.pool.Stats(); queued != 1 {
		t.Fatalf("cancel did not free heap slot: queued=%d", queued)
	}
	if result, _ := f.pool.enqueue(newest, trace.SpanContext{}); result != resultQueued {
		t.Fatal(result)
	}
	f.pool.Cancel(context.Background(), oldest.ID) // Повторная отмена не удаляет чужую задачу.
	gomock.InOrder(
		f.gateway.EXPECT().RequestPosition(gomock.Any(), middle.DriverID, middle.ID).Return(nil),
		f.gateway.EXPECT().RequestPosition(gomock.Any(), newest.DriverID, newest.ID).Return(nil),
	)
	f.start(t, context.Background())
}

func TestPriorityQueueBreaksTimestampTiesByID(t *testing.T) {
	f := newFixture(t, 1, 2)
	first, second := staleTrip(), staleTrip()
	first.ID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	second.ID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	first.PositionAt, second.PositionAt = time.Unix(100, 0), time.Unix(100, 0)
	f.pool.enqueue(second, trace.SpanContext{})
	f.pool.enqueue(first, trace.SpanContext{})
	gomock.InOrder(
		f.gateway.EXPECT().RequestPosition(gomock.Any(), first.DriverID, first.ID).Return(nil),
		f.gateway.EXPECT().RequestPosition(gomock.Any(), second.DriverID, second.ID).Return(nil),
	)
	f.start(t, context.Background())
}

func TestPriorityQueueEvictsMaximumNotLastHeapElement(t *testing.T) {
	f := newFixture(t, 1, 3)
	first, middle, newest := staleTrip(), staleTrip(), staleTrip()
	for _, item := range []trip.StaleTrip{first, newest, middle} {
		f.pool.enqueue(item, trace.SpanContext{})
	}
	incoming := staleTrip()
	incoming.PositionAt = first.PositionAt.Add(-time.Second)
	result, evicted := f.pool.enqueue(incoming, trace.SpanContext{})
	if result != resultQueued || evicted == nil || evicted.trip.ID != newest.ID {
		t.Fatalf("replacement = %q, %+v; want newest trip evicted", result, evicted)
	}
	gomock.InOrder(
		f.gateway.EXPECT().RequestPosition(gomock.Any(), incoming.DriverID, incoming.ID).Return(nil),
		f.gateway.EXPECT().RequestPosition(gomock.Any(), first.DriverID, first.ID).Return(nil),
		f.gateway.EXPECT().RequestPosition(gomock.Any(), middle.DriverID, middle.ID).Return(nil),
	)
	f.start(t, context.Background())
}

func TestConcurrentPriorityQueueWithRunningWorkers(t *testing.T) {
	f := newFixture(t, 4, 10)
	f.gateway.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	f.start(t, context.Background())
	trips := make([]trip.StaleTrip, 50)
	for i := range trips {
		trips[i] = staleTrip()
	}
	var wg sync.WaitGroup
	for n := range 8 {
		wg.Go(func() {
			for i := range 200 {
				item := trips[(i+n)%len(trips)]
				f.pool.enqueue(item, trace.SpanContext{})
				f.pool.Cancel(context.Background(), trips[(i*7+n)%len(trips)].ID)
				queued, active := f.pool.Stats()
				if queued > 10 || active > 4 {
					t.Errorf("pool exceeded bounds: queued=%d active=%d", queued, active)
				}
			}
		})
	}
	wg.Wait()
}
