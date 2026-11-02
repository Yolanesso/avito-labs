package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type UseCase struct {
	repository repository
	txManager  txManager
	metrics    metrics
}

func New(repository repository, txManager txManager, metrics metrics) *UseCase {
	return &UseCase{repository: repository, txManager: txManager, metrics: metrics}
}

func (u *UseCase) Run(ctx context.Context, tripID uuid.UUID) (trip.Trip, error) {
	finishedAt := time.Now().UTC()
	var finishedTrip trip.Trip

	err := u.txManager.Do(ctx, func(ctx context.Context) error {
		var err error
		finishedTrip, err = u.repository.GetByID(ctx, tripID)
		if err != nil {
			return err
		}
		if err := finishedTrip.Finish(finishedAt); err != nil {
			return err
		}
		if err := u.repository.Finish(ctx, tripID, finishedAt); err != nil {
			return err
		}
		return u.repository.AddCompletedHistory(ctx, tripID)
	})
	if err != nil {
		return trip.Trip{}, err
	}

	if u.metrics != nil {
		u.metrics.Completed(ctx, finishedTrip.ID)
	}
	return finishedTrip, nil
}
