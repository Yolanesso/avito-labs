package positions

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type UseCase struct {
	repository repository
	txManager  txManager
	queue      canceller
	metrics    metrics
}

func New(repository repository, txManager txManager, queue canceller, metrics metrics) *UseCase {
	return &UseCase{repository: repository, txManager: txManager, queue: queue, metrics: metrics}
}

func (u *UseCase) Save(ctx context.Context, p trip.Position) (trip.Position, error) {
	var saved trip.Position
	err := u.txManager.Do(ctx, func(ctx context.Context) error {
		if err := u.repository.TouchPosition(ctx, p.TripID, p.RecordedAt); err != nil {
			return err
		}
		var err error
		saved, err = u.repository.InsertPosition(ctx, p)
		return err
	})
	if err != nil {
		return trip.Position{}, err
	}
	// Отменяем только после commit: неуспешная запись не должна отменять опрос.
	u.queue.Cancel(ctx, p.TripID)
	u.metrics.Saved(ctx)
	return saved, nil
}

func (u *UseCase) List(ctx context.Context, tripID uuid.UUID) ([]trip.Position, error) {
	if _, err := u.repository.GetByID(ctx, tripID); err != nil {
		return nil, err
	}
	return u.repository.ListPositions(ctx, tripID)
}
