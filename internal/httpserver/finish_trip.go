package httpserver

import (
	"net/http"

	api "github.com/Yolanesso/avito-labs/internal/generated"
)

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.finish.Handle(w, r, tripID)
}
