//go:build integration

package repository_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	"github.com/Yolanesso/avito-labs/internal/model"
	"github.com/Yolanesso/avito-labs/internal/repository"
	"github.com/Yolanesso/avito-labs/internal/txmanager"
)

type positionDB struct {
	pool *pgxpool.Pool
	repo *repository.TripRepository
	tx   *txmanager.Manager
	ctx  context.Context
}

func newPositionDB(t *testing.T) *positionDB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &positionDB{pool, repository.NewTripRepository(pool, 5*time.Second), txmanager.New(pool), ctx}
}

func (d *positionDB) create(t *testing.T, started time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	err := d.repo.Create(d.ctx, model.Trip{
		ID: id, UserID: uuid.New(), DriverID: uuid.New(), Status: trip.StatusActive,
		StartedAt: started, CreatedAt: started, UpdatedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := d.pool.Exec(d.ctx, "DELETE FROM trips WHERE id = $1", id); err != nil {
			t.Error(err)
		}
	})
	return id
}

func (d *positionDB) save(p trip.Position) (trip.Position, error) {
	var saved trip.Position
	err := d.tx.Do(d.ctx, func(ctx context.Context) error {
		if err := d.repo.TouchPosition(ctx, p.TripID, p.RecordedAt); err != nil {
			return err
		}
		var err error
		saved, err = d.repo.InsertPosition(ctx, p)
		return err
	})
	return saved, err
}

func TestFindStaleTrips(t *testing.T) {
	d := newPositionDB(t)
	now := time.Now().UTC()
	without := d.create(t, now.Add(-10*time.Minute))
	old := d.create(t, now.Add(-20*time.Minute))
	fresh := d.create(t, now.Add(-30*time.Minute))
	recent := d.create(t, now)
	completed := d.create(t, now.Add(-time.Hour))
	for id, at := range map[uuid.UUID]time.Time{old: now.Add(-2 * time.Minute), fresh: now} {
		if _, err := d.save(trip.Position{TripID: id, RecordedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.repo.Finish(d.ctx, completed, now); err != nil {
		t.Fatal(err)
	}
	found, err := d.repo.FindStaleTrips(d.ctx, time.Minute, 10000, nil)
	if err != nil {
		t.Fatal(err)
	}
	// В общей БД могут жить чужие поездки: проверяем только собственные ID.
	ours := map[uuid.UUID]bool{without: true, old: true, fresh: true, recent: true, completed: true}
	var ids []uuid.UUID
	for _, item := range found {
		if ours[item.ID] {
			ids = append(ids, item.ID)
			if item.DriverID == uuid.Nil {
				t.Error("missing driver")
			}
		}
	}
	if len(ids) != 2 || ids[0] != without || ids[1] != old {
		t.Fatalf("stale trips = %v, want oldest without position then old position", ids)
	}
}

func TestCountActiveTrips(t *testing.T) {
	d := newPositionDB(t)
	before, err := d.repo.CountActive(d.ctx)
	if err != nil {
		t.Fatal(err)
	}
	first := d.create(t, time.Now())
	d.create(t, time.Now())
	count, err := d.repo.CountActive(d.ctx)
	if err != nil || count != before+2 {
		t.Fatalf("active count = %d, %v; want %d", count, err, before+2)
	}
	if err := d.repo.Finish(d.ctx, first, time.Now()); err != nil {
		t.Fatal(err)
	}
	count, err = d.repo.CountActive(d.ctx)
	if err != nil || count != before+1 {
		t.Fatalf("active count after finish = %d, %v; want %d", count, err, before+1)
	}
}

func TestPositionsMonotonicTimestampAndRouteOrder(t *testing.T) {
	d := newPositionDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := d.create(t, now.Add(-time.Hour))
	fresh, err := d.save(trip.Position{TripID: id, RecordedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	old, err := d.save(trip.Position{TripID: id, RecordedAt: now.Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	equal, err := d.save(trip.Position{TripID: id, RecordedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	value, err := d.repo.GetByID(d.ctx, id)
	if err != nil || value.LastPositionAt == nil || !value.LastPositionAt.Equal(now) {
		t.Fatalf("last position = %v, %v", value.LastPositionAt, err)
	}
	route, err := d.repo.ListPositions(d.ctx, id)
	if err != nil || len(route) != 3 {
		t.Fatalf("route = %v, %v", route, err)
	}
	if route[0].ID != old.ID || route[1].ID != fresh.ID || route[2].ID != equal.ID {
		t.Fatalf("wrong route order: %v", route)
	}
}

func TestPositionRollbackOnInsertAndAfterInsert(t *testing.T) {
	d := newPositionDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := d.create(t, now.Add(-time.Hour))
	if _, err := d.save(trip.Position{TripID: id, RecordedAt: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	for _, failInsert := range []bool{true, false} {
		err := d.tx.Do(d.ctx, func(ctx context.Context) error {
			if err := d.repo.TouchPosition(ctx, id, now); err != nil {
				return err
			}
			p := trip.Position{TripID: id, RecordedAt: now}
			if failInsert {
				p.Coordinates.Latitude = 91 // настоящий CHECK violation
			}
			if _, err := d.repo.InsertPosition(ctx, p); err != nil {
				return err
			}
			return errors.New("fail after insert")
		})
		if err == nil {
			t.Fatal("expected transaction failure")
		}
		value, err := d.repo.GetByID(d.ctx, id)
		if err != nil || value.LastPositionAt == nil || !value.LastPositionAt.Equal(now.Add(-time.Minute)) {
			t.Fatalf("timestamp changed after rollback: %v, %v", value.LastPositionAt, err)
		}
		route, err := d.repo.ListPositions(d.ctx, id)
		if err != nil || len(route) != 1 {
			t.Fatalf("insert survived rollback: %v, %v", route, err)
		}
	}
}

func TestPositionRejectsMissingAndCompletedTrips(t *testing.T) {
	d := newPositionDB(t)
	now := time.Now()
	id := d.create(t, now.Add(-time.Minute))
	if err := d.repo.Finish(d.ctx, id, now); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[uuid.UUID]error{id: trip.ErrTripCompleted, uuid.New(): trip.ErrTripNotFound} {
		_, err := d.save(trip.Position{TripID: id, RecordedAt: now})
		if !errors.Is(err, want) {
			t.Fatalf("save error = %v, want %v", err, want)
		}
	}
}

func TestConcurrentPositionsKeepNewestTimestamp(t *testing.T) {
	d := newPositionDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := d.create(t, now.Add(-time.Hour))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range 20 {
		wg.Go(func() {
			<-start
			if _, err := d.save(trip.Position{TripID: id, RecordedAt: now.Add(-time.Duration(i) * time.Second)}); err != nil {
				t.Error(err)
			}
		})
	}
	close(start)
	wg.Wait()
	value, err := d.repo.GetByID(d.ctx, id)
	if err != nil || value.LastPositionAt == nil || !value.LastPositionAt.Equal(now) {
		t.Fatalf("last position = %v, %v", value.LastPositionAt, err)
	}
	route, err := d.repo.ListPositions(d.ctx, id)
	if err != nil || len(route) != 20 {
		t.Fatalf("positions = %d, %v", len(route), err)
	}
}

func TestStaleTripsKeysetPagination(t *testing.T) {
	d := newPositionDB(t)
	started := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	expected := make([]trip.StaleTrip, 250)
	ours := make(map[uuid.UUID]bool, len(expected))
	for i := range expected {
		// Много одинаковых времён: продолжение только по timestamp теряло бы строки.
		at := started.Add(time.Duration(i/50) * time.Second)
		id := d.create(t, at)
		if i%3 == 0 {
			at = started.Add(30 * time.Minute)
			if _, err := d.save(trip.Position{TripID: id, RecordedAt: at}); err != nil {
				t.Fatal(err)
			}
		}
		expected[i] = trip.StaleTrip{ID: id, PositionAt: at}
		ours[id] = true
	}
	slices.SortFunc(expected, func(a, b trip.StaleTrip) int {
		if order := a.PositionAt.Compare(b.PositionAt); order != 0 {
			return order
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	var after *trip.StaleTripCursor
	found := make([]trip.StaleTrip, 0, len(expected))
	for {
		batch, err := d.repo.FindStaleTrips(d.ctx, time.Minute, 100, after)
		if err != nil {
			t.Fatal(err)
		}
		if len(batch) > 100 {
			t.Fatalf("batch exceeded LIMIT: %d", len(batch))
		}
		for _, item := range batch {
			if after != nil && (item.PositionAt.Before(after.PositionAt) ||
				(item.PositionAt.Equal(after.PositionAt) && bytes.Compare(item.ID[:], after.ID[:]) <= 0)) {
				t.Fatalf("cursor did not advance: %+v after %+v", item, after)
			}
			if ours[item.ID] {
				found = append(found, item)
			}
		}
		if len(batch) < 100 {
			break
		}
		last := batch[len(batch)-1]
		after = &trip.StaleTripCursor{PositionAt: last.PositionAt, ID: last.ID}
	}
	if len(found) != len(expected) {
		t.Fatalf("paginated trips = %d, want %d", len(found), len(expected))
	}
	for i, item := range found {
		if item.ID != expected[i].ID || !item.PositionAt.Equal(expected[i].PositionAt) {
			t.Fatalf("trip %d = %+v, want %+v", i, item, expected[i])
		}
	}
	if _, err := d.repo.FindStaleTrips(d.ctx, time.Minute, 0, nil); err == nil {
		t.Fatal("expected invalid batch size error")
	}
}

func TestConcurrentFinishAndPosition(t *testing.T) {
	d := newPositionDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	for range 20 {
		id := d.create(t, now.Add(-time.Hour))
		start := make(chan struct{})
		saved := make(chan error, 1)
		finished := make(chan error, 1)
		go func() {
			<-start
			_, err := d.save(trip.Position{TripID: id, RecordedAt: now})
			saved <- err
		}()
		go func() {
			<-start
			finished <- d.repo.Finish(d.ctx, id, now)
		}()
		close(start)
		saveErr, finishErr := <-saved, <-finished
		if finishErr != nil {
			t.Fatal(finishErr)
		}
		value, err := d.repo.GetByID(d.ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if value.Status != trip.StatusCompleted {
			t.Fatalf("status = %q, want completed", value.Status)
		}
		route, err := d.repo.ListPositions(d.ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case saveErr == nil:
			// Сохранение успело захватить строку до завершения.
			if len(route) != 1 || value.LastPositionAt == nil || !value.LastPositionAt.Equal(now) {
				t.Fatalf("committed position: route=%v, last_position_at=%v", route, value.LastPositionAt)
			}
		case errors.Is(saveErr, trip.ErrTripCompleted):
			// Завершение выиграло гонку: ни точка, ни её время не записаны.
			if len(route) != 0 || value.LastPositionAt != nil {
				t.Fatalf("rejected position persisted: route=%v, last_position_at=%v", route, value.LastPositionAt)
			}
		default:
			t.Fatal(saveErr)
		}
	}
}
