package finish_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Yolanesso/avito-labs/internal/httpserver/trip/finish"
	"github.com/Yolanesso/avito-labs/internal/model"
)

type serviceStub struct{}

func (serviceStub) Finish(context.Context, uuid.UUID) (model.Trip, error) {
	return model.Trip{}, model.ErrTripCompleted
}

func TestAlreadyCompleted(t *testing.T) {
	handler := finish.New(serviceStub{})
	tripID := uuid.New()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
	recorder := httptest.NewRecorder()
	handler.Handle(recorder, request, tripID)
	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "trip_completed") {
		t.Errorf("problem code missing: %s", recorder.Body.String())
	}
}
