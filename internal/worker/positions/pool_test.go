package positions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type fixture struct {
	pool     *Pool
	repo     *Mockrepository
	gateway  *Mockgateway
	ticks    chan time.Time
	checks   chan string
	requests chan string
	rows     chan int
}

func newFixture(t *testing.T, workers, size int) *fixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &fixture{
		repo: NewMockrepository(ctrl), gateway: NewMockgateway(ctrl),
		ticks: make(chan time.Time, 10), checks: make(chan string, 1000),
		requests: make(chan string, 10000),
		rows:     make(chan int, 100),
	}
	ticker := NewMockTicker(ctrl)
	ticker.EXPECT().C().Return(f.ticks).AnyTimes()
	ticker.EXPECT().Stop()
	metrics := NewMockmetrics(ctrl)
	metrics.EXPECT().Check(gomock.Any(), gomock.Any()).Do(func(_ context.Context, result string) {
		f.checks <- result
	}).AnyTimes()
	metrics.EXPECT().Request(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, result string, _ time.Duration) {
		f.requests <- result
	}).AnyTimes()
	metrics.EXPECT().RowsRead(gomock.Any(), gomock.Any()).Do(func(_ context.Context, count int) {
		f.rows <- count
	}).AnyTimes()
	f.pool = New(Config{
		Workers: workers, QueueSize: size, BatchSize: 100, StaleAfter: time.Minute, RequestTimeout: time.Second,
	}, f.repo, f.gateway, metrics, slog.New(slog.NewJSONHandler(io.Discard, nil)), ticker)
	return f
}

func (f *fixture) start(t *testing.T, ctx context.Context) {
	t.Helper()
	f.pool.Start(ctx)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := f.pool.Shutdown(ctx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})
}

func receive[T any](t *testing.T, channel <-chan T) T {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for component")
		var zero T
		return zero
	}
}

func assertResult(t *testing.T, channel <-chan string, want string) {
	t.Helper()
	if got := receive(t, channel); got != want {
		t.Fatalf("result = %q, want %q", got, want)
	}
}

var tripSequence atomic.Int64

func staleTrip() trip.StaleTrip {
	return trip.StaleTrip{ID: uuid.New(), DriverID: uuid.New(), PositionAt: time.Unix(tripSequence.Add(1), 0)}
}

func TestRepeatedTickDeduplicatesAndPositionCancelsQueuedTask(t *testing.T) {
	f := newFixture(t, 1, 2)
	first, second := staleTrip(), staleTrip()
	started, release := make(chan struct{}), make(chan struct{})
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).
		Return([]trip.StaleTrip{first, second}, nil).Times(2)
	f.gateway.EXPECT().RequestPosition(gomock.Any(), first.DriverID, first.ID).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			close(started)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	f.start(t, context.Background())
	f.ticks <- time.Now()
	assertResult(t, f.checks, "queued")
	assertResult(t, f.checks, "queued")
	receive(t, started)
	f.ticks <- time.Now()
	assertResult(t, f.checks, "duplicate")
	assertResult(t, f.checks, "duplicate")
	f.pool.Cancel(context.Background(), second.ID)
	assertResult(t, f.requests, "cancelled")
	close(release)
	// Cleanup дожидается очереди; gomock запрещает вызов для second.
}

func TestCancelAndRequeueDoesNotReviveOldTask(t *testing.T) {
	f := newFixture(t, 1, 2)
	job := staleTrip()
	if got, _ := f.pool.enqueue(job, trace.SpanContext{}); got != resultQueued {
		t.Fatal(got)
	}
	f.pool.Cancel(context.Background(), job.ID)
	if got, _ := f.pool.enqueue(job, trace.SpanContext{}); got != resultQueued {
		t.Fatal(got)
	}
	f.gateway.EXPECT().RequestPosition(gomock.Any(), job.DriverID, job.ID).Return(nil).Times(1)
	f.start(t, context.Background())
}

func TestCancelCountsOnlyQueuedTasksOnce(t *testing.T) {
	for _, state := range []string{"queued", "active", "missing"} {
		t.Run(state, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			metrics := NewMockmetrics(ctrl)
			p := New(Config{QueueSize: 1}, nil, nil, metrics, slog.Default(), nil)
			job := staleTrip()
			ctx := context.Background()
			if state != "missing" {
				p.enqueue(job, trace.SpanContext{})
			}
			if state == "active" {
				p.next(ctx)
			}
			if state == "queued" {
				metrics.EXPECT().Request(ctx, "cancelled", time.Duration(0)).Times(1)
			}
			p.Cancel(ctx, job.ID)
			p.Cancel(ctx, job.ID)
			if queued, _ := p.Stats(); queued != 0 {
				t.Fatalf("cancelled task remains queued: %d", queued)
			}
		})
	}
}

func TestConcurrentCancelCountsTaskOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockmetrics(ctrl)
	p := New(Config{QueueSize: 1}, nil, nil, metrics, slog.Default(), nil)
	job := staleTrip()
	p.enqueue(job, trace.SpanContext{})
	ctx := context.Background()
	metrics.EXPECT().Request(ctx, "cancelled", time.Duration(0)).Times(1)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { p.Cancel(ctx, job.ID) })
	}
	wg.Wait()
}

func TestFullQueueDropsWithoutBlockingScanner(t *testing.T) {
	f := newFixture(t, 1, 1)
	first, second, third := staleTrip(), staleTrip(), staleTrip()
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).Return([]trip.StaleTrip{first}, nil)
	f.gateway.EXPECT().RequestPosition(gomock.Any(), first.DriverID, first.ID).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			close(started)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	f.gateway.EXPECT().RequestPosition(gomock.Any(), second.DriverID, second.ID).Return(nil)
	f.start(t, context.Background())
	f.ticks <- time.Now()
	assertResult(t, f.checks, "queued")
	receive(t, started)
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).Return([]trip.StaleTrip{second, third}, nil).Times(2)
	f.ticks <- time.Now()
	assertResult(t, f.checks, "queued")
	assertResult(t, f.checks, "dropped")
	assertResult(t, f.requests, "dropped")
	// Ещё один тик проходит, хотя Push всё ещё заблокирован.
	f.ticks <- time.Now()
	assertResult(t, f.checks, "duplicate")
	assertResult(t, f.checks, "dropped")
	assertResult(t, f.requests, "dropped")
}

func TestFixedWorkersDrainAllTasksOnContextCancel(t *testing.T) {
	const workers, total = 3, 30
	f := newFixture(t, workers, total)
	trips := make([]trip.StaleTrip, total)
	for i := range trips {
		trips[i] = staleTrip()
	}
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).Return(trips, nil)
	started, release := make(chan struct{}, total), make(chan struct{})
	var active, maximum, processed atomic.Int64
	f.gateway.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			current := active.Add(1)
			for old := maximum.Load(); current > old && !maximum.CompareAndSwap(old, current); old = maximum.Load() {
			}
			defer active.Add(-1)
			started <- struct{}{}
			select {
			case <-release:
				processed.Add(1)
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}).Times(total)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.start(t, ctx)
	f.ticks <- time.Now()
	for range total {
		assertResult(t, f.checks, "queued")
	}
	for range workers {
		receive(t, started)
	}
	if _, active := f.pool.Stats(); active != workers {
		t.Fatalf("active = %d, want %d", active, workers)
	}
	cancel()
	close(release)
	receive(t, f.pool.done)
	if processed.Load() != total || maximum.Load() != workers {
		t.Fatalf("processed = %d, max workers = %d", processed.Load(), maximum.Load())
	}
	if queued, active := f.pool.Stats(); queued != 0 || active != 0 {
		t.Fatalf("after shutdown: queued = %d, active = %d", queued, active)
	}
}

func TestPushErrorDoesNotStopWorker(t *testing.T) {
	f := newFixture(t, 1, 2)
	first, second := staleTrip(), staleTrip()
	f.pool.enqueue(first, trace.SpanContext{})
	f.pool.enqueue(second, trace.SpanContext{})
	gomock.InOrder(
		f.gateway.EXPECT().RequestPosition(gomock.Any(), first.DriverID, first.ID).Return(errors.New("push unavailable")),
		f.gateway.EXPECT().RequestPosition(gomock.Any(), second.DriverID, second.ID).Return(nil),
	)
	f.start(t, context.Background())
	assertResult(t, f.requests, "error")
	assertResult(t, f.requests, "success")
}

func TestShutdownDeadlineCancelsInflightAndDiscardsRemaining(t *testing.T) {
	f := newFixture(t, 1, 2)
	first, second := staleTrip(), staleTrip()
	f.pool.enqueue(first, trace.SpanContext{})
	f.pool.enqueue(second, trace.SpanContext{})
	started := make(chan struct{})
	f.gateway.EXPECT().RequestPosition(gomock.Any(), first.DriverID, first.ID).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	f.start(t, context.Background())
	receive(t, started)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.pool.Shutdown(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown error = %v", err)
	}
	receive(t, f.pool.done)
}

func TestScannerQueriesDoNotOverlapAndCancelStopsQuery(t *testing.T) {
	f := newFixture(t, 2, 2)
	started := make(chan struct{})
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).
		DoAndReturn(func(ctx context.Context, _ time.Duration, _ int, _ *trip.StaleTripCursor) ([]trip.StaleTrip, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}).Times(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.start(t, ctx)
	f.ticks <- time.Now()
	receive(t, started)
	f.ticks <- time.Now()
	cancel()
	receive(t, f.pool.done)
}

func TestConcurrentCancelAndEnqueue(t *testing.T) {
	f := newFixture(t, 1, 100)
	job := staleTrip()
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for range 100 {
				f.pool.enqueue(job, trace.SpanContext{})
				f.pool.Cancel(context.Background(), job.ID)
			}
		})
	}
	wg.Wait()
	f.pool.Cancel(context.Background(), job.ID)
	f.start(t, context.Background())
}

func TestWorkerSpanLinksToScanInsteadOfParent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(old)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	f := newFixture(t, 1, 1)
	job := staleTrip()
	f.repo.EXPECT().FindStaleTrips(gomock.Any(), time.Minute, 100, nil).Return([]trip.StaleTrip{job}, nil)
	f.gateway.EXPECT().RequestPosition(gomock.Any(), job.DriverID, job.ID).Return(nil)
	f.start(t, context.Background())
	f.ticks <- time.Now()
	assertResult(t, f.checks, "queued")
	assertResult(t, f.requests, "success")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := f.pool.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	var scan, worker *tracetest.SpanStub
	spans := exporter.GetSpans()
	for i := range spans {
		switch spans[i].Name {
		case "positions.scan":
			scan = &spans[i]
		case "positions.request":
			worker = &spans[i]
		}
	}
	if scan == nil || worker == nil {
		t.Fatalf("missing spans: %v", spans)
	}
	if worker.Parent.IsValid() || worker.SpanContext.TraceID() == scan.SpanContext.TraceID() ||
		len(worker.Links) != 1 || !worker.Links[0].SpanContext.Equal(scan.SpanContext) {
		t.Fatalf("worker must be a new root linked to scan: %+v", worker)
	}
}
