package trip

import (
	"time"

	"github.com/google/uuid"
)

type Position struct {
	ID          int64
	TripID      uuid.UUID
	Coordinates Coordinates
	RecordedAt  time.Time
}

// StaleTrip содержит только поля, нужные для запроса координат у водителя.
type StaleTrip struct {
	ID         uuid.UUID
	DriverID   uuid.UUID
	PositionAt time.Time // COALESCE(last_position_at, started_at), ключ приоритета.
}

// StaleTripCursor продолжает выборку строго после последней прочитанной строки.
type StaleTripCursor struct {
	PositionAt time.Time
	ID         uuid.UUID
}
