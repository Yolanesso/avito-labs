package create_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	"github.com/Yolanesso/avito-labs/internal/usecase/trip/create"
)

type repositoryStub struct {
	saved       trip.Trip
	historyID   uuid.UUID
	createError error
}

func (r *repositoryStub) Create(_ context.Context, value trip.Trip) error {
	r.saved = value
	return r.createError
}

func (r *repositoryStub) AddCreatedHistory(_ context.Context, id uuid.UUID) error {
	r.historyID = id
	return nil
}

type txStub struct{}

func (txStub) Do(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type metricsStub struct{ createdID uuid.UUID }

func (m *metricsStub) Created(_ context.Context, id uuid.UUID) {
	m.createdID = id
}

func TestCreate(t *testing.T) {
	repo := &repositoryStub{}
	metrics := &metricsStub{}
	userID := uuid.New()
	driverID := uuid.New()
	uc := create.New(repo, txStub{}, metrics)

	got, err := uc.Run(context.Background(), create.Input{
		UserID: userID, DriverID: driverID, Price: 1450,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if got.ID == uuid.Nil || got.ID != repo.saved.ID || got.ID != repo.historyID {
		t.Errorf("trip and history should have the same non-empty ID: %+v", got)
	}
	if got.Status != trip.StatusActive || got.UserID != userID || got.DriverID != driverID {
		t.Errorf("unexpected trip: %+v", got)
	}
	if metrics.createdID != got.ID {
		t.Errorf("event ID = %s, want %s", metrics.createdID, got.ID)
	}
}

func TestCreateErrorDoesNotEmitEvent(t *testing.T) {
	repo := &repositoryStub{createError: trip.ErrDriverBusy}
	metrics := &metricsStub{}
	uc := create.New(repo, txStub{}, metrics)

	_, err := uc.Run(context.Background(), create.Input{DriverID: uuid.New()})
	if !errors.Is(err, trip.ErrDriverBusy) {
		t.Errorf("Run() error = %v, want ErrDriverBusy", err)
	}
	if repo.historyID != uuid.Nil || metrics.createdID != uuid.Nil {
		t.Error("history or event appeared after failed creation")
	}
}
