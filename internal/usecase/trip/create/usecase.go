package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type Input struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint trip.Coordinates
	EndPoint   trip.Coordinates
	Price      int64
}

type UseCase struct {
	repository repository
	txManager  txManager
	metrics    metrics
}

func New(repository repository, txManager txManager, metrics metrics) *UseCase {
	return &UseCase{repository: repository, txManager: txManager, metrics: metrics}
}

func (u *UseCase) Run(ctx context.Context, input Input) (trip.Trip, error) {
	now := time.Now().UTC()
	newTrip := trip.Trip{
		ID:         uuid.New(),
		UserID:     input.UserID,
		DriverID:   input.DriverID,
		StartPoint: input.StartPoint,
		EndPoint:   input.EndPoint,
		Price:      input.Price,
		Status:     trip.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := u.txManager.Do(ctx, func(ctx context.Context) error {
		if err := u.repository.Create(ctx, newTrip); err != nil {
			return err
		}
		return u.repository.AddCreatedHistory(ctx, newTrip.ID)
	})
	if err != nil {
		return trip.Trip{}, err
	}

	if u.metrics != nil {
		u.metrics.Created(ctx, newTrip.ID)
	}
	return newTrip, nil
}
