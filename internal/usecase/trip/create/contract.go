//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create_test

package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type repository interface {
	Create(context.Context, trip.Trip) error
	AddCreatedHistory(context.Context, uuid.UUID) error
}

type txManager interface {
	Do(context.Context, func(context.Context) error) error
}

type metrics interface {
	Created(context.Context, uuid.UUID)
}
