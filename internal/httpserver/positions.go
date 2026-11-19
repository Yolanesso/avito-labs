package httpserver

import (
	"net/http"

	api "github.com/Yolanesso/avito-labs/internal/generated"
)

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.positions.Save(w, r, tripID)
}

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.positions.List(w, r, tripID)
}
