//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=positions

package positions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
)

type repository interface {
	FindStaleTrips(context.Context, time.Duration, int, *trip.StaleTripCursor) ([]trip.StaleTrip, error)
}

type gateway interface {
	RequestPosition(context.Context, uuid.UUID, uuid.UUID) error
}

type metrics interface {
	Check(context.Context, string)
	Request(context.Context, string, time.Duration)
	RowsRead(context.Context, int)
}

type Ticker interface {
	C() <-chan time.Time
	Stop()
}
