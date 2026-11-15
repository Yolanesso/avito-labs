package domainerror

import (
	"errors"
	"net/http"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	api "github.com/Yolanesso/avito-labs/internal/generated"
)

func Problem(err error) (int, api.Problem) {
	status := http.StatusInternalServerError
	code := "internal_error"
	title := "Internal error"
	detail := "Internal server error"
	problemType := "https://tripgo.example/problems/internal-error"

	switch {
	case errors.Is(err, trip.ErrTripNotFound):
		status = http.StatusNotFound
		code = "trip_not_found"
		title = "Trip not found"
		detail = "Trip with this ID does not exist"
		problemType = "https://tripgo.example/problems/trip-not-found"

	case errors.Is(err, trip.ErrTripCompleted):
		status = http.StatusConflict
		code = "trip_completed"
		title = "Trip completed"
		detail = "Trip is already completed"
		problemType = "https://tripgo.example/problems/trip-completed"

	case errors.Is(err, trip.ErrDriverBusy):
		status = http.StatusConflict
		code = "driver_busy"
		title = "Driver busy"
		detail = "Driver already has an active trip"
		problemType = "https://tripgo.example/problems/driver-busy"
	}

	return status, api.Problem{
		Type:   problemType,
		Title:  title,
		Status: int32(status),
		Detail: &detail,
		Code:   code,
	}
}
