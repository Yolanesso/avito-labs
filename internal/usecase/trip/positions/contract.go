//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=positions

package positions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type repository interface {
	GetByID(context.Context, uuid.UUID) (trip.Trip, error)
	TouchPosition(context.Context, uuid.UUID, time.Time) error
	InsertPosition(context.Context, trip.Position) (trip.Position, error)
	ListPositions(context.Context, uuid.UUID) ([]trip.Position, error)
}

type txManager interface {
	Do(context.Context, func(context.Context) error) error
}

type canceller interface {
	Cancel(context.Context, uuid.UUID)
}

type metrics interface {
	Saved(context.Context)
}
