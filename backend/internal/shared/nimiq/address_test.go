package nimiq

import "testing"

func TestNormalizeUserFriendlyAddressAcceptsOfficialTestAddress(t *testing.T) {
	got, err := NormalizeUserFriendlyAddress("nq46klje5tmf4y1a1255cjhjyg1sh0nut604")
	if err != nil {
		t.Fatalf("NormalizeUserFriendlyAddress() error = %v", err)
	}
	if got != "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604" {
		t.Fatalf("NormalizeUserFriendlyAddress() = %q", got)
	}
}

func TestNormalizeUserFriendlyAddressRejectsInvalidChecksum(t *testing.T) {
	if _, err := NormalizeUserFriendlyAddress("NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T605"); err == nil {
		t.Fatal("expected checksum rejection")
	}
}
