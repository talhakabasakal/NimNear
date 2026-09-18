package nimiq

import (
	"fmt"
	"strings"
)

const userFriendlyAlphabet = "0123456789ABCDEFGHJKLMNPQRSTUVXY"

// NormalizeUserFriendlyAddress validates the Nimiq IBAN checksum and returns
// the canonical spaced NQ address. Format-only checks are not sufficient.
func NormalizeUserFriendlyAddress(value string) (string, error) {
	compact := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	if len(compact) != 36 || !strings.HasPrefix(compact, "NQ") {
		return "", fmt.Errorf("invalid address length or prefix")
	}
	for _, c := range compact[4:] {
		if !strings.ContainsRune(userFriendlyAlphabet, c) {
			return "", fmt.Errorf("invalid address alphabet")
		}
	}
	if ibanMod97(compact[4:]+compact[:4]) != 1 {
		return "", fmt.Errorf("invalid address checksum")
	}
	return formatUserFriendlyAddress(compact), nil
}

func ibanMod97(value string) int {
	remainder := 0
	for _, c := range value {
		if c >= '0' && c <= '9' {
			remainder = (remainder*10 + int(c-'0')) % 97
		} else if c >= 'A' && c <= 'Z' {
			n := int(c-'A') + 10
			remainder = (remainder*100 + n) % 97
		} else {
			return -1
		}
	}
	return remainder
}

func formatUserFriendlyAddress(compact string) string {
	var parts []string
	for i := 0; i < len(compact); i += 4 {
		parts = append(parts, compact[i:i+4])
	}
	return strings.Join(parts, " ")
}
