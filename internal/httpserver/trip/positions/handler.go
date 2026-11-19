package positions

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/domain/trip"
	api "github.com/Yolanesso/avito-labs/internal/generated"
	"github.com/Yolanesso/avito-labs/internal/httpserver/response"
)

type Handler struct {
	service service
	now     func() time.Time
}

func New(service service, now func() time.Time) *Handler {
	return &Handler{service: service, now: now}
}

func (h *Handler) Save(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) {
	// Указатели позволяют отличить пропущенные обязательные поля от нулевых координат.
	var request struct {
		Coordinates *struct {
			Latitude  *float64 `json:"latitude"`
			Longitude *float64 `json:"longitude"`
		} `json:"coordinates"`
		RecordedAt *time.Time `json:"recorded_at"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		response.InvalidRequest(w, r, "Invalid JSON body")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		response.InvalidRequest(w, r, "Request body must contain one JSON object")
		return
	}
	if request.Coordinates == nil || request.Coordinates.Latitude == nil || request.Coordinates.Longitude == nil ||
		request.RecordedAt == nil || request.RecordedAt.IsZero() {
		response.InvalidRequest(w, r, "coordinates and recorded_at are required")
		return
	}
	latitude, longitude := *request.Coordinates.Latitude, *request.Coordinates.Longitude
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		response.InvalidRequest(w, r, "Coordinates are out of range")
		return
	}
	if request.RecordedAt.After(h.now().Add(time.Minute)) {
		response.InvalidRequest(w, r, "recorded_at is more than one minute in the future")
		return
	}
	saved, err := h.service.Save(r.Context(), trip.Position{
		TripID:      tripID,
		Coordinates: trip.Coordinates{Latitude: latitude, Longitude: longitude},
		RecordedAt:  *request.RecordedAt,
	})
	if err != nil {
		response.DomainError(w, r, err)
		return
	}
	response.JSON(w, http.StatusCreated, toAPI(saved))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) {
	positions, err := h.service.List(r.Context(), tripID)
	if err != nil {
		response.DomainError(w, r, err)
		return
	}
	result := make([]api.TripPosition, 0, len(positions))
	for _, p := range positions {
		result = append(result, toAPI(p))
	}
	response.JSON(w, http.StatusOK, api.TripPositionsResponse{Positions: result})
}

func toAPI(p trip.Position) api.TripPosition {
	return api.TripPosition{
		Id: p.ID, TripId: p.TripID, RecordedAt: p.RecordedAt,
		Coordinates: api.Coordinates{Latitude: p.Coordinates.Latitude, Longitude: p.Coordinates.Longitude},
	}
}
