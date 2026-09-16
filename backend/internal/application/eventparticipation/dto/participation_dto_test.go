package dto

import (
	"encoding/json"
	"testing"
)

func TestParticipationInfoUnlimitedCapacityUsesExplicitNull(t *testing.T) {
	payload, err := json.Marshal(ParticipationInfo{})
	if err != nil {
		t.Fatalf("marshal ParticipationInfo: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal ParticipationInfo JSON: %v", err)
	}
	value, ok := fields["capacity"]
	if !ok || string(value) != "null" {
		t.Fatalf("capacity = %s (present %v), want explicit null", value, ok)
	}
}
