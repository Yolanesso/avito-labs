package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	api "github.com/Yolanesso/avito-labs/internal/generated"
	requestmiddleware "github.com/Yolanesso/avito-labs/internal/httpserver/middleware"
	"github.com/Yolanesso/avito-labs/internal/httpserver/response"
)

type Timeouts struct {
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func New(
	addr string,
	handler api.ServerInterface,
	logger *slog.Logger,
	timeouts Timeouts,
) *http.Server {
	router := chi.NewRouter()

	router.Use(recoverMiddleware(logger))
	router.Use(requestmiddleware.Tracing)
	router.Use(requestmiddleware.Logging(logger))
	router.Use(requestmiddleware.Metrics(router, logger))

	api.HandlerWithOptions(
		handler,
		api.ChiServerOptions{
			BaseRouter: router,

			ErrorHandlerFunc: func(
				w http.ResponseWriter,
				r *http.Request,
				err error,
			) {
				response.InvalidRequest(w, r, err.Error())
			},
		},
	)

	return &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadTimeout:       timeouts.ReadTimeout,
		ReadHeaderTimeout: timeouts.ReadHeaderTimeout,
		WriteTimeout:      timeouts.WriteTimeout,
		IdleTimeout:       timeouts.IdleTimeout,
	}
}
