//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=finish_test

package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type repository interface {
	GetByID(context.Context, uuid.UUID) (trip.Trip, error)
	Finish(context.Context, uuid.UUID, time.Time) error
	AddCompletedHistory(context.Context, uuid.UUID) error
}

type txManager interface {
	Do(context.Context, func(context.Context) error) error
}

type metrics interface {
	Completed(context.Context, uuid.UUID)
}
