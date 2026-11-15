//go:build integration

package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Yolanesso/avito-labs/internal/model"
	"github.com/Yolanesso/avito-labs/internal/repository"
	"github.com/Yolanesso/avito-labs/internal/txmanager"
)

func TestRollbackTripAndHistory(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}

	tripID := uuid.New()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cleanupCancel()

		_, err := pool.Exec(cleanupCtx, "DELETE FROM trips WHERE id = $1", tripID)
		if err != nil {
			t.Errorf("clean up trip: %v", err)
		}
		pool.Close()
	})

	now := time.Now().UTC()
	trip := model.Trip{
		ID:        tripID,
		UserID:    uuid.New(),
		DriverID:  uuid.New(),
		Price:     100,
		Status:    model.TripStatusActive,
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	repo := repository.NewTripRepository(pool, 5*time.Second)
	tx := txmanager.New(pool)
	rollbackError := errors.New("force rollback")

	err = tx.Do(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, trip); err != nil {
			return err
		}
		if err := repo.AddCreatedHistory(ctx, tripID); err != nil {
			return err
		}

		return rollbackError
	})
	if !errors.Is(err, rollbackError) {
		t.Fatalf("transaction error = %v, want rollbackError", err)
	}

	var tripsCount int
	err = pool.QueryRow(
		ctx, "SELECT count(*) FROM trips WHERE id = $1", tripID,
	).Scan(&tripsCount)
	if err != nil {
		t.Fatalf("count trips: %v", err)
	}

	var historyCount int
	err = pool.QueryRow(
		ctx, "SELECT count(*) FROM trip_status_history WHERE trip_id = $1", tripID,
	).Scan(&historyCount)
	if err != nil {
		t.Fatalf("count history: %v", err)
	}

	if tripsCount != 0 || historyCount != 0 {
		t.Errorf("after rollback: trips = %d, history = %d; want 0 and 0",
			tripsCount, historyCount)
	}
}

func TestCreateTripAndHistory(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}

	tripID := uuid.New()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cleanupCancel()

		_, err := pool.Exec(cleanupCtx, "DELETE FROM trips WHERE id = $1", tripID)
		if err != nil {
			t.Errorf("clean up trip: %v", err)
		}
		pool.Close()
	})

	now := time.Now().UTC()
	trip := model.Trip{
		ID:        tripID,
		UserID:    uuid.New(),
		DriverID:  uuid.New(),
		Price:     100,
		Status:    model.TripStatusActive,
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	repo := repository.NewTripRepository(pool, 5*time.Second)
	tx := txmanager.New(pool)

	err = tx.Do(ctx, func(ctx context.Context) error {
		if err := repo.Create(ctx, trip); err != nil {
			return err
		}
		return repo.AddCreatedHistory(ctx, tripID)
	})
	if err != nil {
		t.Fatalf("save trip and history: %v", err)
	}

	var status string
	err = pool.QueryRow(
		ctx, "SELECT status FROM trips WHERE id = $1", tripID,
	).Scan(&status)
	if err != nil {
		t.Fatalf("read trip: %v", err)
	}
	if status != "active" {
		t.Errorf("status = %q, want active", status)
	}

	var historyCount int
	err = pool.QueryRow(
		ctx, "SELECT count(*) FROM trip_status_history WHERE trip_id = $1", tripID,
	).Scan(&historyCount)
	if err != nil {
		t.Fatalf("count history: %v", err)
	}
	if historyCount != 1 {
		t.Errorf("history rows = %d, want 1", historyCount)
	}
}
