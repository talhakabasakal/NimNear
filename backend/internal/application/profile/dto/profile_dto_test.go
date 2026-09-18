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

func TestPublicProfileInfoOptionalFieldsUseExplicitNull(t *testing.T) {
	payload, err := json.Marshal(PublicProfileInfo{})
	if err != nil {
		t.Fatalf("marshal PublicProfileInfo: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal PublicProfileInfo JSON: %v", err)
	}
	for _, name := range []string{"username", "bio", "avatar_url", "wallet_address"} {
		value, ok := fields[name]
		if !ok {
			t.Fatalf("optional field %q was omitted", name)
		}
		if string(value) != "null" {
			t.Fatalf("optional field %q = %s, want null", name, value)
		}
	}
}

func TestPublicProfileInfoOptionalFieldsPreserveValues(t *testing.T) {
	username, bio, avatarURL := "nimnear_user", "About", "https://media.example/avatar.png"
	payload, err := json.Marshal(PublicProfileInfo{Username: &username, Bio: &bio, AvatarURL: &avatarURL})
	if err != nil {
		t.Fatalf("marshal PublicProfileInfo: %v", err)
	}
	var decoded PublicProfileInfo
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal PublicProfileInfo: %v", err)
	}
	if decoded.Username == nil || *decoded.Username != username || decoded.Bio == nil || *decoded.Bio != bio || decoded.AvatarURL == nil || *decoded.AvatarURL != avatarURL {
		t.Fatalf("optional values were not preserved: %#v", decoded)
	}
}
