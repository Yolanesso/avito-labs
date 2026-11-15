package get_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/httpserver/trip/get"
	"github.com/Yolanesso/avito-labs/internal/model"
)

type serviceStub struct{}

func (serviceStub) GetByID(context.Context, uuid.UUID) (model.Trip, error) {
	return model.Trip{}, model.ErrTripNotFound
}

func TestNotFound(t *testing.T) {
	handler := get.New(serviceStub{})
	tripID := uuid.New()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String(), nil)
	recorder := httptest.NewRecorder()
	handler.Handle(recorder, request, tripID)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "trip_not_found") {
		t.Errorf("problem code missing: %s", recorder.Body.String())
	}
}
