package create_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/Yolanesso/avito-labs/internal/httpserver/trip/create"
)

func TestInvalidJSONWithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := NewMocktripCreator(ctrl)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader("{"))
	recorder := httptest.NewRecorder()
	create.New(svc).Handle(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", recorder.Code)
	}
	// На битом JSON сервис не вызывается: у мока нет ожидаемых вызовов.
}
