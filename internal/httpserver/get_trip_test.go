package httpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	api "github.com/Yolanesso/avito-labs/internal/generated"
	"github.com/Yolanesso/avito-labs/internal/httpserver"
	"github.com/Yolanesso/avito-labs/internal/model"
)

func TestGetTripNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	tripService := NewMocktripService(ctrl)
	tripID := uuid.New()

	tripService.EXPECT().
		GetByID(gomock.Any(), tripID).
		Return(model.Trip{}, model.ErrTripNotFound)

	handler := httpserver.NewHandler(
		func(context.Context) error { return nil },
		time.Second,
		tripService,
	)

	server := httpserver.New(":0", handler, slog.Default())

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/trips/"+tripID.String(),
		nil,
	)
	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}

	var problem api.Problem
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if problem.Code != "trip_not_found" {
		t.Errorf("code = %q, want trip_not_found", problem.Code)
	}
}

func TestCreateTripInvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	tripService := NewMocktripService(ctrl)

	handler := httpserver.NewHandler(
		func(context.Context) error { return nil },
		time.Second,
		tripService,
	)
	server := httpserver.New(":0", handler, slog.Default())

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips",
		strings.NewReader("{invalid"),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}

	var problem api.Problem
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if problem.Code != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", problem.Code)
	}
}

func TestGetTripInternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	tripService := NewMocktripService(ctrl)
	tripID := uuid.New()

	tripService.EXPECT().
		GetByID(gomock.Any(), tripID).
		Return(model.Trip{}, errors.New("database password is secret"))

	handler := httpserver.NewHandler(
		func(context.Context) error { return nil },
		time.Second,
		tripService,
	)
	server := httpserver.New(":0", handler, slog.Default())

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/trips/"+tripID.String(),
		nil,
	)
	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "database password") {
		t.Fatal("internal error details leaked to the client")
	}

	var problem api.Problem
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if problem.Code != "internal_error" {
		t.Errorf("code = %q, want internal_error", problem.Code)
	}
}
