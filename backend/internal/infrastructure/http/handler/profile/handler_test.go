package profile

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateMeRequiresAuthentication(t *testing.T) {
	handler := NewHandler(nil)
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/me/profile", nil)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.UpdateMe(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
