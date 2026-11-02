package finish

import (
	"net/http"

	api "github.com/Yolanesso/avito-labs/internal/generated"
	"github.com/Yolanesso/avito-labs/internal/httpserver/response"
)

type Handler struct {
	service tripFinisher
}

func New(service tripFinisher) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	value, err := h.service.Finish(r.Context(), tripID)
	if err != nil {
		response.DomainError(w, r, err)
		return
	}
	response.JSON(w, http.StatusOK, response.TripToAPI(value))
}
