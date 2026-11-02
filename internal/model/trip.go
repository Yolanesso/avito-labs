package model

import "github.com/Yolanesso/avito-labs/internal/domain/trip"

type TripStatus = trip.Status

const (
	TripStatusActive    = trip.StatusActive
	TripStatusCompleted = trip.StatusCompleted
)

type (
	Coordinates = trip.Coordinates
	Trip        = trip.Trip
)
