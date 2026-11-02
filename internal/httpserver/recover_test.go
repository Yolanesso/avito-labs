package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	handler := recoverMiddleware(logger)(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("do not show me")
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "do not show me") {
		t.Fatal("panic details leaked to the client")
	}
}
