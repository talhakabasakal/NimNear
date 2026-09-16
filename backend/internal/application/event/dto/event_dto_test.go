package dto

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestEventInfoOptionalFieldsUseExplicitNull(t *testing.T) {
	payload, err := json.Marshal(EventInfo{})
	if err != nil {
		t.Fatalf("marshal EventInfo: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal EventInfo JSON: %v", err)
	}
	for _, name := range []string{"capacity", "image_url", "calendar_id", "place_id", "latitude", "longitude", "address", "organizer_id"} {
		value, ok := fields[name]
		if !ok {
			t.Fatalf("optional field %q was omitted", name)
		}
		if string(value) != "null" {
			t.Fatalf("optional field %q = %s, want null", name, value)
		}
	}
}

func TestEventInfoOptionalFieldsPreserveValues(t *testing.T) {
	capacity := 20
	imageURL := "https://media.example/event.jpg"
	placeID := uuid.New()
	latitude, longitude := 41.0, 29.0
	address := "Galata"
	organizerID := uuid.New()
	calendarID := uuid.New()
	event := EventInfo{Capacity: &capacity, ImageURL: &imageURL, CalendarID: &calendarID, PlaceID: &placeID, Latitude: &latitude, Longitude: &longitude, Address: &address, OrganizerID: &organizerID}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal EventInfo: %v", err)
	}
	var decoded EventInfo
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal EventInfo: %v", err)
	}
	if decoded.Capacity == nil || *decoded.Capacity != capacity || decoded.ImageURL == nil || *decoded.ImageURL != imageURL || decoded.CalendarID == nil || *decoded.CalendarID != calendarID || decoded.PlaceID == nil || *decoded.PlaceID != placeID || decoded.Latitude == nil || *decoded.Latitude != latitude || decoded.Longitude == nil || *decoded.Longitude != longitude || decoded.Address == nil || *decoded.Address != address || decoded.OrganizerID == nil || *decoded.OrganizerID != organizerID {
		t.Fatalf("optional values were not preserved: %#v", decoded)
	}
}
