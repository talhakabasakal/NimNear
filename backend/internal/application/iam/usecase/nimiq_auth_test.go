package usecase

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type authRepoStub struct {
	challenge *NimiqChallenge
	consumed  bool
	user      *model.User
}

func (r *authRepoStub) CreateChallenge(_ context.Context, c *NimiqChallenge) error {
	r.challenge = c
	return nil
}
func (r *authRepoStub) GetChallenge(_ context.Context, _ uuid.UUID) (*NimiqChallenge, error) {
	if r.challenge == nil {
		return nil, domainErr.New(domainErr.ErrNotFound, "missing", nil)
	}
	return r.challenge, nil
}
func (r *authRepoStub) ConsumeAndResolveIdentity(_ context.Context, _ uuid.UUID, _ []byte, _ time.Time, accountLabel string) (*model.User, error) {
	if r.consumed {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "challenge_already_used", "used", nil)
	}
	r.consumed = true
	if r.user != nil && accountLabel != "" && r.user.FirstName == "" {
		r.user.FirstName = accountLabel
	}
	return r.user, nil
}
func (r *authRepoStub) AddressForUser(context.Context, uuid.UUID) (string, error) { return "", nil }
func (r *authRepoStub) VerifiedAddresses(context.Context, uuid.UUID) ([]string, error) {
	return nil, nil
}

type authServiceStub struct{}

func (authServiceStub) HashPassword(string) (string, error) { return "", nil }
func (authServiceStub) VerifyPassword(string, string) error { return nil }
func (authServiceStub) GenerateToken(context.Context, service.TokenClaims) (string, error) {
	return "signed-session", nil
}
func (authServiceStub) ValidateToken(context.Context, string) (*service.TokenClaims, error) {
	return nil, errors.New("unused")
}

func testAuthConfig() config.NimiqAuthConfig {
	return config.NimiqAuthConfig{Network: "test-albatross", Environment: "testnet", Domain: "test.nimnear.local", ChallengeTTL: 5 * time.Minute, CookieName: "nimnear_session", CookieSameSite: "lax"}
}

func TestAddressFromPublicKeyOfficialVector(t *testing.T) {
	key, err := hex.DecodeString("03a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8")
	if err != nil {
		t.Fatal(err)
	}
	got := AddressFromPublicKey(key)
	want := "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"
	if got != want {
		t.Fatalf("AddressFromPublicKey()=%q want %q", got, want)
	}
	if normalized, err := NormalizeNimiqAddress("nq46klje5tmf4y1a1255cjhjyg1sh0nut604"); err != nil || normalized != want {
		t.Fatalf("NormalizeNimiqAddress()=%q,%v", normalized, err)
	}
}

func TestCreateChallengeBindsCanonicalMessage(t *testing.T) {
	repo := &authRepoStub{}
	uc := NewNimiqAuthUseCase(repo, authServiceStub{}, testAuthConfig())
	fixed := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	uc.now = func() time.Time { return fixed }
	response, err := uc.CreateChallenge(context.Background(), NimiqChallengeRequest{Address: "NQ46KLJE5TMF4Y1A1255CJHJYG1SH0NUT604", Network: "test-albatross", Environment: "testnet", Purpose: "AUTH_LOGIN", Transport: "mini-app"})
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"domain: test.nimnear.local", "audience: nimnear-api", "environment: testnet", "network: test-albatross", "address: NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604", "purpose: AUTH_LOGIN"} {
		if !contains(response.Message, part) {
			t.Errorf("message missing %q", part)
		}
	}
	if response.Message != repo.challenge.Message {
		t.Fatal("response must contain the exact stored canonical message")
	}
	if !response.ExpiresAt.Equal(fixed.Add(5 * time.Minute)) {
		t.Fatal("wrong expiry")
	}
}

func TestCreateChallengeAcceptsNormalizedPaymentSpelling(t *testing.T) {
	cfg := testAuthConfig()
	cfg.Network = "TestAlbatross"
	response, err := NewNimiqAuthUseCase(&authRepoStub{}, authServiceStub{}, cfg).CreateChallenge(context.Background(), NimiqChallengeRequest{
		Address: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604", Network: "test-albatross", Environment: "testnet", Purpose: "AUTH_LOGIN", Transport: "hub",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Network != "test-albatross" || response.Environment != "testnet" {
		t.Fatalf("canonical challenge network=%q environment=%q", response.Network, response.Environment)
	}
}

func TestCreateChallengeRejectsUnknownNetwork(t *testing.T) {
	req := NimiqChallengeRequest{Address: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604", Network: "ethereum", Environment: "testnet", Purpose: "AUTH_LOGIN", Transport: "hub"}
	if _, err := NewNimiqAuthUseCase(&authRepoStub{}, authServiceStub{}, testAuthConfig()).CreateChallenge(context.Background(), req); err == nil {
		t.Fatal("expected unknown network rejection")
	}
}

func TestCreateChallengeRejectsDeploymentMismatch(t *testing.T) {
	cases := []NimiqChallengeRequest{
		{Address: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604", Network: "main-albatross", Environment: "testnet", Purpose: "AUTH_LOGIN", Transport: "hub"},
		{Address: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604", Network: "test-albatross", Environment: "mainnet", Purpose: "AUTH_LOGIN", Transport: "hub"},
	}
	for _, req := range cases {
		if _, err := NewNimiqAuthUseCase(&authRepoStub{}, authServiceStub{}, testAuthConfig()).CreateChallenge(context.Background(), req); err == nil {
			t.Fatal("expected mismatch rejection")
		}
	}
}

func TestVerifyAcceptsBothOfficialTransportRepresentations(t *testing.T) {
	for _, transport := range []string{NimiqTransportMiniApp, NimiqTransportHub} {
		t.Run(transport, func(t *testing.T) {
			public, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
			message := "NIMNear canonical ASCII challenge"
			challenge := &NimiqChallenge{ID: uuid.New(), Purpose: NimiqAuthPurpose, Transport: transport, Environment: "testnet", Network: "test-albatross", Domain: "test.nimnear.local", Audience: NimiqAuthAudience, ClaimedAddress: AddressFromPublicKey(public), Message: message, IssuedAt: now, ExpiresAt: now.Add(time.Minute)}
			user := &model.User{ID: uuid.New(), Status: model.UserStatusActive, CreatedAt: now}
			repo := &authRepoStub{challenge: challenge, user: user}
			uc := NewNimiqAuthUseCase(repo, authServiceStub{}, testAuthConfig())
			uc.now = func() time.Time { return now }
			preimage, err := SignedBytes(transport, message)
			if err != nil {
				t.Fatal(err)
			}
			signature := ed25519.Sign(private, preimage)
			result, err := uc.Verify(context.Background(), NimiqVerifyRequest{ChallengeID: challenge.ID, Message: message, PublicKey: hex.EncodeToString(public), Signature: hex.EncodeToString(signature)})
			if err != nil {
				t.Fatal(err)
			}
			if result.Token != "signed-session" || result.User.ID != user.ID {
				t.Fatalf("unexpected session: %#v", result)
			}
			if result.User.WalletAddress != challenge.ClaimedAddress {
				t.Fatalf("wallet address missing from session: %#v", result.User)
			}
			if result.User.DisplayName != challenge.ClaimedAddress {
				t.Fatalf("display name should fall back to wallet address, got %q", result.User.DisplayName)
			}
		})
	}
}

func TestVerifyUsesHubAccountLabelAsDisplayName(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	message := "NIMNear canonical ASCII challenge"
	address := AddressFromPublicKey(public)
	challenge := &NimiqChallenge{ID: uuid.New(), Purpose: NimiqAuthPurpose, Transport: NimiqTransportHub, Environment: "testnet", Network: "test-albatross", Domain: "test.nimnear.local", Audience: NimiqAuthAudience, ClaimedAddress: address, Message: message, IssuedAt: now, ExpiresAt: now.Add(time.Minute)}
	user := &model.User{ID: uuid.New(), Status: model.UserStatusActive, CreatedAt: now}
	repo := &authRepoStub{challenge: challenge, user: user}
	uc := NewNimiqAuthUseCase(repo, authServiceStub{}, testAuthConfig())
	uc.now = func() time.Time { return now }
	preimage, err := SignedBytes(NimiqTransportHub, message)
	if err != nil {
		t.Fatal(err)
	}
	result, err := uc.Verify(context.Background(), NimiqVerifyRequest{
		ChallengeID:  challenge.ID,
		Message:      message,
		PublicKey:    hex.EncodeToString(public),
		Signature:    hex.EncodeToString(ed25519.Sign(private, preimage)),
		AccountLabel: "Teal Address",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.User.DisplayName != "Teal Address" || result.User.WalletAddress != address {
		t.Fatalf("expected Hub account on session, got %#v", result.User)
	}
}

func TestSanitizeAccountLabelRejectsControlCharacters(t *testing.T) {
	if got := SanitizeAccountLabel("  Teal Address  "); got != "Teal Address" {
		t.Fatalf("SanitizeAccountLabel()=%q", got)
	}
	if SanitizeAccountLabel("bad\nlabel") != "" || SanitizeAccountLabel(strings.Repeat("a", 101)) != "" {
		t.Fatal("invalid labels must be ignored")
	}
}

func TestVerifyRejectsTamperingExpiryReplayAndAddressMismatch(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	otherPublic, _, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().UTC()
	message := "challenge"
	validSignature := func() string {
		b, _ := SignedBytes(NimiqTransportMiniApp, message)
		return hex.EncodeToString(ed25519.Sign(private, b))
	}
	makeUC := func() *NimiqAuthUseCase {
		c := &NimiqChallenge{ID: uuid.New(), Purpose: NimiqAuthPurpose, Transport: NimiqTransportMiniApp, Environment: "testnet", Network: "test-albatross", Domain: "test.nimnear.local", Audience: NimiqAuthAudience, ClaimedAddress: AddressFromPublicKey(public), Message: message, IssuedAt: now, ExpiresAt: now.Add(time.Minute)}
		u := NewNimiqAuthUseCase(&authRepoStub{challenge: c, user: &model.User{ID: uuid.New(), Status: model.UserStatusActive}}, authServiceStub{}, testAuthConfig())
		u.now = func() time.Time { return now }
		return u
	}
	tests := []struct {
		name   string
		mutate func(*NimiqAuthUseCase, *NimiqVerifyRequest)
	}{
		{"message", func(_ *NimiqAuthUseCase, r *NimiqVerifyRequest) { r.Message = "tampered" }},
		{"address", func(_ *NimiqAuthUseCase, r *NimiqVerifyRequest) { r.PublicKey = hex.EncodeToString(otherPublic) }},
		{"signature", func(_ *NimiqAuthUseCase, r *NimiqVerifyRequest) { r.Signature = hex.EncodeToString(make([]byte, 64)) }},
		{"expired", func(u *NimiqAuthUseCase, _ *NimiqVerifyRequest) {
			u.now = func() time.Time { return now.Add(2 * time.Minute) }
		}},
		{"consumed", func(u *NimiqAuthUseCase, _ *NimiqVerifyRequest) {
			x := now
			u.repo.(*authRepoStub).challenge.ConsumedAt = &x
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := makeUC()
			r := NimiqVerifyRequest{ChallengeID: u.repo.(*authRepoStub).challenge.ID, Message: message, PublicKey: hex.EncodeToString(public), Signature: validSignature()}
			tc.mutate(u, &r)
			if _, err := u.Verify(context.Background(), r); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestNormalizeNimiqAddressRejectsInvalidValues(t *testing.T) {
	valid := "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"
	normalized, err := NormalizeNimiqAddress(valid)
	if err != nil || normalized != valid {
		t.Fatalf("valid address rejected: %q %v", normalized, err)
	}

	invalid := []string{
		"",
		"   ",
		"NQ...",
		"random-string",
		"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T605",
		"NQ46KLJE5TMF4Y1A1255CJHJYG1SH0NUT60",
		"ETHEREUM0XABC",
	}
	for _, value := range invalid {
		if _, err := NormalizeNimiqAddress(value); err == nil {
			t.Errorf("NormalizeNimiqAddress(%q) accepted invalid address", value)
		}
	}
}

func contains(value, part string) bool {
	return len(value) >= len(part) && func() bool {
		for i := 0; i+len(part) <= len(value); i++ {
			if value[i:i+len(part)] == part {
				return true
			}
		}
		return false
	}()
}
