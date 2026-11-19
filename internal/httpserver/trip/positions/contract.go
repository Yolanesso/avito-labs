//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=positions_test

package positions

import (
	"context"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type service interface {
	Save(context.Context, trip.Position) (trip.Position, error)
	List(context.Context, uuid.UUID) ([]trip.Position, error)
}
