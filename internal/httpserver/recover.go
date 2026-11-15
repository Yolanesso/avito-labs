package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/Yolanesso/avito-labs/internal/httpserver/response"
)

func recoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if panicValue := recover(); panicValue != nil {
					logger.Error(
						"panic recovered",
						"panic", panicValue,
						"stack", string(debug.Stack()),
					)

					response.DomainError(w, r, errors.New("panic recovered"))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
