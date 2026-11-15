package httpserver

import (
	"net/http"

	api "github.com/Yolanesso/avito-labs/internal/generated"
)

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	h.create.Handle(w, r)
}
