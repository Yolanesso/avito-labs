package httpserver

import (
	"context"
	"net/http"
	"time"

	api "github.com/Yolanesso/avito-labs/internal/generated"
	"github.com/Yolanesso/avito-labs/internal/httpserver/response"
	tripcreate "github.com/Yolanesso/avito-labs/internal/httpserver/trip/create"
	tripfinish "github.com/Yolanesso/avito-labs/internal/httpserver/trip/finish"
	tripget "github.com/Yolanesso/avito-labs/internal/httpserver/trip/get"
)

type Handler struct {
	api.Unimplemented

	pingDatabase func(context.Context) error
	queryTimeout time.Duration
	create       *tripcreate.Handler
	get          *tripget.Handler
	finish       *tripfinish.Handler
}

func NewHandler(
	pingDatabase func(context.Context) error,
	queryTimeout time.Duration,
	tripService tripService,
) *Handler {
	return &Handler{
		pingDatabase: pingDatabase,
		queryTimeout: queryTimeout,
		create:       tripcreate.New(tripService),
		get:          tripget.New(tripService),
		finish:       tripfinish.New(tripService),
	}
}

func (h *Handler) Health(
	w http.ResponseWriter,
	_ *http.Request,
) {
	response.JSON(w, http.StatusOK, api.HealthResponse{
		Status: api.Ok,
	})
}

func (h *Handler) Ready(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx, cancel := context.WithTimeout(
		r.Context(),
		h.queryTimeout,
	)
	defer cancel()

	err := h.pingDatabase(ctx)
	if err != nil {
		response.JSON(
			w,
			http.StatusServiceUnavailable,
			api.HealthResponse{
				Status: api.Unavailable,
			},
		)
		return
	}

	response.JSON(w, http.StatusOK, api.HealthResponse{
		Status: api.Ok,
	})
}
