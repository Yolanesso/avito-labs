//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=get_test

package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type repository interface {
	GetByID(context.Context, uuid.UUID) (trip.Trip, error)
}
