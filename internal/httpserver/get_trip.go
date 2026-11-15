package httpserver

import (
	"net/http"

	api "github.com/Yolanesso/avito-labs/internal/generated"
)

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.get.Handle(w, r, tripID)
}
