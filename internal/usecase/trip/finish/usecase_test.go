package finish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	"github.com/Yolanesso/avito-labs/internal/usecase/trip/finish"
)

type repositoryStub struct {
	value     trip.Trip
	finished  bool
	historyID uuid.UUID
}

func (r *repositoryStub) GetByID(_ context.Context, _ uuid.UUID) (trip.Trip, error) {
	return r.value, nil
}

func (r *repositoryStub) Finish(_ context.Context, _ uuid.UUID, _ time.Time) error {
	r.finished = true
	return nil
}

func (r *repositoryStub) AddCompletedHistory(_ context.Context, id uuid.UUID) error {
	r.historyID = id
	return nil
}

type txStub struct{}

func (txStub) Do(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type metricsStub struct{ completedID uuid.UUID }

func (m *metricsStub) Completed(_ context.Context, id uuid.UUID) {
	m.completedID = id
}

func TestFinishActiveTrip(t *testing.T) {
	id := uuid.New()
	repo := &repositoryStub{value: trip.Trip{ID: id, Status: trip.StatusActive}}
	metrics := &metricsStub{}
	uc := finish.New(repo, txStub{}, metrics)
	got, err := uc.Run(context.Background(), id)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if got.Status != trip.StatusCompleted || got.FinishedAt == nil {
		t.Errorf("trip was not completed: %+v", got)
	}
	if !repo.finished || repo.historyID != id || metrics.completedID != id {
		t.Error("database update, history and event must all happen")
	}
}

func TestFinishCompletedTrip(t *testing.T) {
	id := uuid.New()
	repo := &repositoryStub{value: trip.Trip{ID: id, Status: trip.StatusCompleted}}
	metrics := &metricsStub{}
	uc := finish.New(repo, txStub{}, metrics)
	_, err := uc.Run(context.Background(), id)
	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Errorf("Run() error = %v, want ErrTripCompleted", err)
	}
	if repo.finished || repo.historyID != uuid.Nil || metrics.completedID != uuid.Nil {
		t.Error("completed trip was updated or emitted an event")
	}
}
