package get_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/httpserver/trip/get"
	"github.com/Yolanesso/avito-labs/internal/model"
)

func TestGetNotFoundWithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMocktripGetter(ctrl)
	id := uuid.New()
	svc.EXPECT().GetByID(gomock.Any(), id).Return(model.Trip{}, model.ErrTripNotFound)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String(), nil)
	recorder := httptest.NewRecorder()
	get.New(svc).Handle(recorder, request, id)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
}
