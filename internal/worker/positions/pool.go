package positions

import (
	"container/heap"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type Config struct {
	StaleAfter     time.Duration
	Workers        int
	QueueSize      int
	BatchSize      int
	RequestTimeout time.Duration
}

const (
	resultQueued    = "queued"
	resultDuplicate = "duplicate"
	resultDropped   = "dropped"
)

type task struct {
	trip  trip.StaleTrip
	link  trace.SpanContext
	index int // -1, если задача уже выполняется или удалена.
}

type Pool struct {
	cfg        Config
	repository repository
	gateway    gateway
	metrics    metrics
	logger     *slog.Logger
	ticker     Ticker
	queue      taskQueue

	mu        sync.Mutex
	pending   map[uuid.UUID]*task
	active    int64
	available *sync.Cond
	closed    bool

	stopScan   context.CancelFunc
	cancelWork context.CancelFunc
	done       chan struct{}
}

func New(cfg Config, repository repository, gateway gateway, metrics metrics, logger *slog.Logger, ticker Ticker) *Pool {
	p := &Pool{
		cfg: cfg, repository: repository, gateway: gateway, metrics: metrics,
		logger: logger, ticker: ticker, queue: make(taskQueue, 0, cfg.QueueSize),
		pending: make(map[uuid.UUID]*task), done: make(chan struct{}),
	}
	p.available = sync.NewCond(&p.mu)
	return p
}

// Start вызывается один раз. Отмена ctx останавливает сканер, но не обрывает
// задачи: воркеры сначала осушают очередь. Shutdown ограничивает это общим сроком.
func (p *Pool) Start(ctx context.Context) {
	scanCtx, stopScan := context.WithCancel(ctx)
	workCtx, cancelWork := context.WithCancel(context.WithoutCancel(ctx))
	p.stopScan = stopScan
	p.cancelWork = cancelWork
	var workers sync.WaitGroup
	for range p.cfg.Workers {
		workers.Go(func() { p.work(workCtx) })
	}
	go func() {
		defer close(p.done)
		p.logger.InfoContext(scanCtx, "position scanner started")
		p.scan(scanCtx)
		p.ticker.Stop()
		p.logger.InfoContext(scanCtx, "position scanner stopped")
		p.mu.Lock()
		p.closed = true // Единственный производитель уже остановлен.
		p.available.Broadcast()
		p.mu.Unlock()
		workers.Wait()
		cancelWork()
		p.logger.InfoContext(workCtx, "position workers stopped")
	}()
	p.logger.InfoContext(ctx, "position workers started", "workers", p.cfg.Workers)
}

func (p *Pool) Shutdown(ctx context.Context) error {
	p.stopScan()
	select {
	case <-p.done:
		p.cancelWork()
		return nil
	case <-ctx.Done():
		// Общий бюджет исчерпан: отменяем и выполняющиеся HTTP-вызовы.
		p.cancelWork()
		<-p.done
		return ctx.Err()
	}
}

func (p *Pool) Cancel(ctx context.Context, tripID uuid.UUID) {
	p.mu.Lock()
	cancelled := false
	if job := p.pending[tripID]; job != nil && job.index >= 0 {
		heap.Remove(&p.queue, job.index)
		cancelled = true
	}
	delete(p.pending, tripID)
	p.mu.Unlock()
	if cancelled {
		p.metrics.Request(ctx, "cancelled", 0)
	}
}

func (p *Pool) Stats() (queued, active int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return int64(len(p.queue)), p.active
}

func (p *Pool) scan(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.ticker.C():
			if ctx.Err() != nil {
				return
			}
			p.check(ctx)
		}
	}
}

func (p *Pool) check(ctx context.Context) {
	ctx, span := otel.Tracer("tripgo/positions").Start(ctx, "positions.scan")
	defer span.End()
	read := 0
	defer func() { p.metrics.RowsRead(ctx, read) }()
	var after *trip.StaleTripCursor
	for {
		if ctx.Err() != nil {
			return
		}
		trips, err := p.repository.FindStaleTrips(ctx, p.cfg.StaleAfter, p.cfg.BatchSize, after)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "find stale trips")
			if ctx.Err() == nil {
				p.logger.WarnContext(ctx, "find stale trips failed", "error", err)
			}
			return
		}
		read += len(trips)
		for _, t := range trips {
			if ctx.Err() != nil {
				return
			}
			if p.admit(ctx, t) {
				return
			}
		}
		queued, _ := p.Stats()
		if len(trips) < p.cfg.BatchSize || queued >= int64(p.cfg.QueueSize) {
			return
		}
		last := trips[len(trips)-1]
		after = &trip.StaleTripCursor{PositionAt: last.PositionAt, ID: last.ID}
	}
}

// admit сообщает, была ли отброшена задача (включая вытеснение из очереди).
func (p *Pool) admit(ctx context.Context, t trip.StaleTrip) bool {
	result, evicted := p.enqueue(t, trace.SpanContextFromContext(ctx))
	p.metrics.Check(ctx, result)
	if result == resultDropped {
		p.drop(ctx, t.ID)
	}
	if evicted != nil {
		p.metrics.Check(ctx, resultDropped)
		p.drop(ctx, evicted.trip.ID)
	}
	return result == resultDropped || evicted != nil
}

func (p *Pool) drop(ctx context.Context, id uuid.UUID) {
	p.metrics.Request(ctx, resultDropped, 0)
	p.logger.WarnContext(ctx, "position queue full; task dropped", "trip_id", id)
}

func (p *Pool) enqueue(t trip.StaleTrip, link trace.SpanContext) (string, *task) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.pending[t.ID]; exists {
		return resultDuplicate, nil
	}
	job := &task{trip: t, link: link, index: -1}
	var evicted *task
	if len(p.queue) >= p.cfg.QueueSize {
		evicted = p.queue.replaceNewest(job)
		if evicted == nil {
			return resultDropped, nil
		}
		delete(p.pending, evicted.trip.ID)
	} else {
		heap.Push(&p.queue, job)
	}
	p.pending[t.ID] = job
	p.available.Signal()
	return resultQueued, evicted
}

func (p *Pool) next(ctx context.Context) *task {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.queue) == 0 && !p.closed {
		p.available.Wait()
	}
	if ctx.Err() != nil {
		for len(p.queue) > 0 {
			job := heap.Pop(&p.queue).(*task)
			delete(p.pending, job.trip.ID)
		}
		return nil
	}
	if len(p.queue) == 0 {
		return nil
	}
	job := heap.Pop(&p.queue).(*task)
	p.active++
	return job
}

func (p *Pool) work(ctx context.Context) {
	for {
		job := p.next(ctx)
		if job == nil {
			return
		}
		p.process(ctx, job)
		p.mu.Lock()
		p.active--
		// Cancel выполняющегося запроса может разрешить новую постановку того же ID.
		if p.pending[job.trip.ID] == job {
			delete(p.pending, job.trip.ID)
		}
		p.mu.Unlock()
	}
}

func (p *Pool) process(ctx context.Context, job *task) {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()
	ctx, span := otel.Tracer("tripgo/positions").Start(ctx, "positions.request",
		trace.WithNewRoot(), trace.WithLinks(trace.Link{SpanContext: job.link}))
	defer span.End()
	start := time.Now()
	result := "success"
	if err := p.gateway.RequestPosition(ctx, job.trip.DriverID, job.trip.ID); err != nil {
		result = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, "request position")
		p.logger.WarnContext(ctx, "request position failed", "trip_id", job.trip.ID, "error", err)
	}
	p.metrics.Request(ctx, result, time.Since(start))
}
