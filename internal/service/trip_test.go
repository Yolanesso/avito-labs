package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/model"
	"github.com/Yolanesso/avito-labs/internal/service"
)

type tripMetricsStub struct {
	created   []uuid.UUID
	completed []uuid.UUID
}

func (m *tripMetricsStub) Created(_ context.Context, tripID uuid.UUID) {
	m.created = append(m.created, tripID)
}

func (m *tripMetricsStub) Completed(_ context.Context, tripID uuid.UUID) {
	m.completed = append(m.completed, tripID)
}

func TestCreateTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	userID := uuid.New()
	driverID := uuid.New()
	var savedID uuid.UUID

	tx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, got model.Trip) error {
			if got.ID == uuid.Nil {
				t.Error("trip ID is empty")
			}
			if got.Status != model.TripStatusActive {
				t.Errorf("status = %q, want active", got.Status)
			}
			if got.UserID != userID || got.DriverID != driverID {
				t.Error("user or driver ID was not saved")
			}
			savedID = got.ID
			return nil
		})

	repo.EXPECT().
		AddCreatedHistory(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tripID uuid.UUID) error {
			if tripID != savedID {
				t.Errorf("history ID = %s, want %s", tripID, savedID)
			}
			return nil
		})

	svc := service.NewTripService(repo, tx)
	metrics := &tripMetricsStub{}
	svc.SetMetrics(metrics)
	got, err := svc.Create(context.Background(), service.CreateTripInput{
		UserID:   userID,
		DriverID: driverID,
		Price:    1450,
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if got.ID != savedID {
		t.Errorf("returned ID = %s, want %s", got.ID, savedID)
	}
	if len(metrics.created) != 1 || metrics.created[0] != got.ID {
		t.Errorf("created events = %v, want [%s]", metrics.created, got.ID)
	}
}

func TestFinishCompletedTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	tripID := uuid.New()

	tx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	repo.EXPECT().
		GetByID(gomock.Any(), tripID).
		Return(model.Trip{
			ID:     tripID,
			Status: model.TripStatusCompleted,
		}, nil)

	svc := service.NewTripService(repo, tx)
	metrics := &tripMetricsStub{}
	svc.SetMetrics(metrics)
	_, err := svc.Finish(context.Background(), tripID)

	if !errors.Is(err, model.ErrTripCompleted) {
		t.Errorf("Finish() error = %v, want ErrTripCompleted", err)
	}
	if len(metrics.completed) != 0 {
		t.Errorf("completed events = %v, want none", metrics.completed)
	}
}

func TestFinishActiveTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	tripID := uuid.New()

	tx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	repo.EXPECT().
		GetByID(gomock.Any(), tripID).
		Return(model.Trip{
			ID:     tripID,
			Status: model.TripStatusActive,
		}, nil)

	repo.EXPECT().
		Finish(gomock.Any(), tripID, gomock.Any()).
		Return(nil)

	repo.EXPECT().
		AddCompletedHistory(gomock.Any(), tripID).
		Return(nil)

	svc := service.NewTripService(repo, tx)
	metrics := &tripMetricsStub{}
	svc.SetMetrics(metrics)
	got, err := svc.Finish(context.Background(), tripID)
	if err != nil {
		t.Fatalf("Finish() error: %v", err)
	}

	if got.Status != model.TripStatusCompleted {
		t.Errorf("status = %q, want completed", got.Status)
	}
	if got.FinishedAt == nil {
		t.Error("finished_at was not set")
	}
	if len(metrics.completed) != 1 || metrics.completed[0] != tripID {
		t.Errorf("completed events = %v, want [%s]", metrics.completed, tripID)
	}
}

func TestCreateTripDriverBusy(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	tx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(model.ErrDriverBusy)

	svc := service.NewTripService(repo, tx)
	metrics := &tripMetricsStub{}
	svc.SetMetrics(metrics)
	_, err := svc.Create(context.Background(), service.CreateTripInput{
		UserID:   uuid.New(),
		DriverID: uuid.New(),
		Price:    1450,
	})

	if !errors.Is(err, model.ErrDriverBusy) {
		t.Errorf("Create() error = %v, want ErrDriverBusy", err)
	}
	if len(metrics.created) != 0 {
		t.Errorf("created events = %v, want none", metrics.created)
	}
}

func TestGetTripNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	tx := NewMocktxManager(ctrl)

	tripID := uuid.New()

	repo.EXPECT().
		GetByID(gomock.Any(), tripID).
		Return(model.Trip{}, model.ErrTripNotFound)

	svc := service.NewTripService(repo, tx)
	_, err := svc.GetByID(context.Background(), tripID)

	if !errors.Is(err, model.ErrTripNotFound) {
		t.Errorf("GetByID() error = %v, want ErrTripNotFound", err)
	}
}
