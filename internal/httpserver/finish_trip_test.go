package httpserver_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	api "github.com/Yolanesso/avito-labs/internal/generated"
	"github.com/Yolanesso/avito-labs/internal/httpserver"
	"github.com/Yolanesso/avito-labs/internal/model"
)

func TestFinishCompletedTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	tripService := NewMocktripService(ctrl)
	tripID := uuid.New()

	tripService.EXPECT().
		Finish(gomock.Any(), tripID).
		Return(model.Trip{}, model.ErrTripCompleted)

	handler := httpserver.NewHandler(
		func(context.Context) error { return nil },
		time.Second,
		tripService,
	)
	server := httpserver.New(":0", handler, slog.Default())

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/trips/"+tripID.String()+"/finish",
		nil,
	)
	response := httptest.NewRecorder()

	server.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}

	var problem api.Problem
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if problem.Code != "trip_completed" {
		t.Errorf("code = %q, want trip_completed", problem.Code)
	}
}
