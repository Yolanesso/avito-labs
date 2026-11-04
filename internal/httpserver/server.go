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

func New(
	addr string,
	handler api.ServerInterface,
	logger *slog.Logger,
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
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
