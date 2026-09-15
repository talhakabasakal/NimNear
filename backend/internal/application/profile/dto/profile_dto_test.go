package dto

import (
	"encoding/json"
	"testing"
)

func TestUpdateProfileRequestDistinguishesOmittedAndNull(t *testing.T) {
	var request UpdateProfileRequest
	if err := json.Unmarshal([]byte(`{"display_name":null,"bio":"about me"}`), &request); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if !request.DisplayName.Set || request.DisplayName.Value != nil {
		t.Fatalf("display name null was not preserved: %#v", request.DisplayName)
	}
	if !request.Bio.Set || request.Bio.Value == nil || *request.Bio.Value != "about me" {
		t.Fatalf("bio value was not preserved: %#v", request.Bio)
	}
	if request.Username.Set {
		t.Fatal("omitted username was treated as supplied")
	}
}

func TestUpdateProfileRequestRejectsEmptyObject(t *testing.T) {
	var request UpdateProfileRequest
	if err := json.Unmarshal([]byte(`{}`), &request); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if !request.Empty() || !request.Patch().Empty() {
		t.Fatalf("empty request was not detected: %#v", request)
	}
}
