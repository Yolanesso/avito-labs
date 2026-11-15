package model

import "github.com/Yolanesso/avito-labs/internal/domain/trip"

var (
	ErrTripNotFound  = trip.ErrTripNotFound
	ErrTripCompleted = trip.ErrTripCompleted
	ErrDriverBusy    = trip.ErrDriverBusy
)
