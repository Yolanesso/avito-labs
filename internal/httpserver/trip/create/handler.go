package create

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	api "github.com/Yolanesso/avito-labs/internal/generated"
	"github.com/Yolanesso/avito-labs/internal/httpserver/response"
	"github.com/Yolanesso/avito-labs/internal/model"
	"github.com/Yolanesso/avito-labs/internal/service"
)

type Handler struct {
	service tripCreator
}

func New(service tripCreator) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	var request api.CreateTripJSONRequestBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		response.InvalidRequest(w, r, fmt.Sprintf("Invalid JSON body: %v", err))
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		response.InvalidRequest(w, r, "Request body must contain one JSON object")
		return
	}
	if request.UserId == uuid.Nil {
		response.InvalidRequest(w, r, "user_id must not be empty")
		return
	}
	if request.DriverId == uuid.Nil {
		response.InvalidRequest(w, r, "driver_id must not be empty")
		return
	}
	if !coordinatesAreValid(request.StartPoint) {
		response.InvalidRequest(w, r, "start_point contains invalid coordinates")
		return
	}
	if !coordinatesAreValid(request.EndPoint) {
		response.InvalidRequest(w, r, "end_point contains invalid coordinates")
		return
	}
	if request.Price < 0 {
		response.InvalidRequest(w, r, "price must not be negative")
		return
	}

	input := service.CreateTripInput{
		UserID:   request.UserId,
		DriverID: request.DriverId,
		StartPoint: model.Coordinates{
			Latitude:  request.StartPoint.Latitude,
			Longitude: request.StartPoint.Longitude,
		},
		EndPoint: model.Coordinates{
			Latitude:  request.EndPoint.Latitude,
			Longitude: request.EndPoint.Longitude,
		},
		Price: request.Price,
	}

	value, err := h.service.Create(r.Context(), input)
	if err != nil {
		response.DomainError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/trips/"+value.ID.String())
	response.JSON(w, http.StatusCreated, response.TripToAPI(value))
}

func coordinatesAreValid(coordinates api.Coordinates) bool {
	if coordinates.Latitude < -90 || coordinates.Latitude > 90 {
		return false
	}
	if coordinates.Longitude < -180 || coordinates.Longitude > 180 {
		return false
	}
	return true
}
