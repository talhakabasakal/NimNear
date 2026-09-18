package validator

import (
	"net/url"
	"strings"
	"unicode"
)

// MaxMediaURLLength is the maximum byte length accepted for an external media URL.
const MaxMediaURLLength = 2048

// ValidMediaURL accepts an absent URL or an absolute HTTP(S) URL without
// credentials or whitespace. The browser loads approved external media directly.
func ValidMediaURL(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > MaxMediaURLLength || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if unicode.IsSpace(char) || unicode.IsControl(char) {
			return false
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" || parsed.Hostname() == "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https"
}

// ValidHTTPSMediaURL accepts an absent URL or an absolute HTTPS URL that
// already satisfies ValidMediaURL. HTTP is rejected.
func ValidHTTPSMediaURL(value string) bool {
	if value == "" {
		return true
	}
	if !ValidMediaURL(value) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed != nil && strings.EqualFold(parsed.Scheme, "https")
}
