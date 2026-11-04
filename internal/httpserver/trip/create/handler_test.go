package create_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yolanesso/avito-labs/internal/httpserver/trip/create"
	"github.com/Yolanesso/avito-labs/internal/model"
	"github.com/Yolanesso/avito-labs/internal/service"
)

type serviceStub struct{}

func (serviceStub) Create(context.Context, service.CreateTripInput) (model.Trip, error) {
	return model.Trip{}, nil
}

func TestInvalidJSON(t *testing.T) {
	handler := create.New(serviceStub{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader("{broken"))
	recorder := httptest.NewRecorder()
	handler.Handle(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "invalid_request") {
		t.Errorf("problem code missing: %s", recorder.Body.String())
	}
}
