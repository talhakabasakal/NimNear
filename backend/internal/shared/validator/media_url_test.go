package validator

import (
	"strings"
	"testing"
)

func TestValidMediaURL(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "absent", value: "", valid: true},
		{name: "http", value: "http://media.example/event.jpg", valid: true},
		{name: "https", value: "https://media.example/avatar.png?size=2", valid: true},
		{name: "malformed", value: "://missing-scheme", valid: false},
		{name: "javascript", value: "javascript:alert(1)", valid: false},
		{name: "data", value: "data:image/png;base64,AAAA", valid: false},
		{name: "file", value: "file:///tmp/image.png", valid: false},
		{name: "unsupported scheme", value: "ftp://media.example/image.png", valid: false},
		{name: "credentials", value: "https://user:pass@media.example/image.png", valid: false},
		{name: "whitespace", value: "https://media.example/image name.png", valid: false},
		{name: "too long", value: "https://media.example/" + strings.Repeat("a", MaxMediaURLLength), valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidMediaURL(test.value); got != test.valid {
				t.Fatalf("ValidMediaURL(%q) = %v, want %v", test.value, got, test.valid)
			}
		})
	}
}
