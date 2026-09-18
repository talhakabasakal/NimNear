package usecase

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/dto"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	nimiqnet "github.com/masterfabric-go/masterfabric/internal/shared/nimiq"
	"golang.org/x/crypto/blake2b"
)

const (
	NimiqAuthPurpose         = "AUTH_LOGIN"
	NimiqAuthAudience        = "nimnear-api"
	NimiqTransportMiniApp    = "mini-app"
	NimiqTransportHub        = "hub"
	nimiqAlphabet            = "0123456789ABCDEFGHJKLMNPQRSTUVXY"
	nimiqSignedMessagePrefix = "\x16Nimiq Signed Message:\n"
)

type NimiqChallenge struct {
	ID             uuid.UUID
	Nonce          []byte
	Purpose        string
	Transport      string
	Environment    string
	Network        string
	Domain         string
	Audience       string
	ClaimedAddress string
	Message        string
	IssuedAt       time.Time
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
}

type NimiqChallengeRepository interface {
	CreateChallenge(context.Context, *NimiqChallenge) error
	GetChallenge(context.Context, uuid.UUID) (*NimiqChallenge, error)
	ConsumeAndResolveIdentity(context.Context, uuid.UUID, []byte, time.Time, string) (*model.User, error)
	AddressForUser(context.Context, uuid.UUID) (string, error)
	VerifiedAddresses(context.Context, uuid.UUID) ([]string, error)
}

type NimiqChallengeRequest struct {
	Address     string `json:"wallet_address" validate:"required"`
	Network     string `json:"network" validate:"required"`
	Environment string `json:"environment" validate:"required"`
	Purpose     string `json:"purpose" validate:"required"`
	Transport   string `json:"transport" validate:"required"`
}

type NimiqChallengeResponse struct {
	ChallengeID   uuid.UUID `json:"challenge_id"`
	Message       string    `json:"message"`
	WalletAddress string    `json:"wallet_address"`
	Network       string    `json:"network"`
	Environment   string    `json:"environment"`
	Purpose       string    `json:"purpose"`
	Transport     string    `json:"transport"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type NimiqVerifyRequest struct {
	ChallengeID  uuid.UUID `json:"challenge_id" validate:"required"`
	Message      string    `json:"message" validate:"required"`
	PublicKey    string    `json:"public_key" validate:"required"`
	Signature    string    `json:"signature" validate:"required"`
	AccountLabel string    `json:"account_label"`
}

type NimiqAuthUseCase struct {
	repo   NimiqChallengeRepository
	auth   service.AuthService
	config config.NimiqAuthConfig
	now    func() time.Time
}

func NewNimiqAuthUseCase(repo NimiqChallengeRepository, auth service.AuthService, cfg config.NimiqAuthConfig) *NimiqAuthUseCase {
	return &NimiqAuthUseCase{repo: repo, auth: auth, config: cfg, now: func() time.Time { return time.Now().UTC() }}
}

func (uc *NimiqAuthUseCase) CreateChallenge(ctx context.Context, req NimiqChallengeRequest) (*NimiqChallengeResponse, error) {
	address, err := NormalizeNimiqAddress(req.Address)
	if err != nil {
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "invalid_wallet_address", "invalid Nimiq wallet address", nil)
	}
	if req.Purpose != NimiqAuthPurpose {
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "invalid_auth_purpose", "purpose must be AUTH_LOGIN", nil)
	}
	if req.Transport != NimiqTransportMiniApp && req.Transport != NimiqTransportHub {
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "unsupported_auth_transport", "unsupported Nimiq authentication transport", nil)
	}
	configuredNetwork, err := nimiqnet.ParseNetwork(uc.config.Network)
	if err != nil {
		return nil, domainErr.NewWithCode(domainErr.ErrInternal, "unsupported_nimiq_network", "Nimiq authentication network is not configured", nil)
	}
	requestedNetwork, err := nimiqnet.ParseNetwork(req.Network)
	if err != nil || requestedNetwork.ID != configuredNetwork.ID {
		return nil, networkMismatchError("unsupported_nimiq_network", "Nimiq network does not match this deployment", req.Network, req.Environment, configuredNetwork)
	}
	if req.Environment != configuredNetwork.Environment {
		return nil, networkMismatchError("environment_mismatch", "authentication environment does not match this deployment", req.Network, req.Environment, configuredNetwork)
	}

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to generate authentication challenge", err)
	}
	issuedAt := uc.now()
	expiresAt := issuedAt.Add(uc.config.ChallengeTTL)
	nonceText := base64.RawURLEncoding.EncodeToString(nonce)
	message := fmt.Sprintf("NIMNear Nimiq authentication\nversion: 1\ndomain: %s\naudience: %s\nenvironment: %s\nnetwork: %s\naddress: %s\npurpose: %s\nchallenge: %s\nissued_at: %s\nexpires_at: %s", uc.config.Domain, NimiqAuthAudience, configuredNetwork.Environment, configuredNetwork.AuthName, address, NimiqAuthPurpose, nonceText, issuedAt.Format(time.RFC3339), expiresAt.Format(time.RFC3339))
	challenge := &NimiqChallenge{ID: uuid.New(), Nonce: nonce, Purpose: NimiqAuthPurpose, Transport: req.Transport, Environment: configuredNetwork.Environment, Network: configuredNetwork.AuthName, Domain: uc.config.Domain, Audience: NimiqAuthAudience, ClaimedAddress: address, Message: message, IssuedAt: issuedAt, ExpiresAt: expiresAt}
	if err := uc.repo.CreateChallenge(ctx, challenge); err != nil {
		return nil, err
	}
	return &NimiqChallengeResponse{ChallengeID: challenge.ID, Message: challenge.Message, WalletAddress: address, Network: challenge.Network, Environment: challenge.Environment, Purpose: challenge.Purpose, Transport: challenge.Transport, IssuedAt: issuedAt, ExpiresAt: expiresAt}, nil
}

func (uc *NimiqAuthUseCase) Verify(ctx context.Context, req NimiqVerifyRequest) (*dto.LoginResponse, error) {
	challenge, err := uc.repo.GetChallenge(ctx, req.ChallengeID)
	if err != nil {
		return nil, err
	}
	now := uc.now()
	if challenge.ConsumedAt != nil {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "challenge_already_used", "authentication challenge was already used", nil)
	}
	if !now.Before(challenge.ExpiresAt) {
		return nil, domainErr.NewWithCode(domainErr.ErrGone, "challenge_expired", "authentication challenge expired", nil)
	}
	if challenge.Message != req.Message {
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "challenge_message_mismatch", "signed message does not match the server challenge", nil)
	}
	if !nimiqnet.SameEnvironment(challenge.Network, uc.config.Network) {
		configuredNetwork, parseErr := nimiqnet.ParseNetwork(uc.config.Network)
		if parseErr != nil {
			return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "network_mismatch", "authentication challenge network mismatch", nil)
		}
		return nil, networkMismatchError("network_mismatch", "authentication challenge network mismatch", challenge.Network, challenge.Environment, configuredNetwork)
	}
	configuredNetwork, err := nimiqnet.ParseNetwork(uc.config.Network)
	if err != nil || challenge.Environment != configuredNetwork.Environment || challenge.Domain != uc.config.Domain || challenge.Audience != NimiqAuthAudience || challenge.Purpose != NimiqAuthPurpose {
		if err == nil && challenge.Environment != configuredNetwork.Environment {
			return nil, networkMismatchError("environment_mismatch", "authentication challenge deployment binding mismatch", challenge.Network, challenge.Environment, configuredNetwork)
		}
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "environment_mismatch", "authentication challenge deployment binding mismatch", nil)
	}

	publicKey, err := decodeFixedHex(req.PublicKey, ed25519.PublicKeySize)
	if err != nil || allZero(publicKey) {
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "invalid_public_key", "invalid Nimiq public key", nil)
	}
	signature, err := decodeFixedHex(req.Signature, ed25519.SignatureSize)
	if err != nil {
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "invalid_signature_encoding", "invalid Nimiq signature encoding", nil)
	}
	derivedAddress := AddressFromPublicKey(publicKey)
	if derivedAddress != challenge.ClaimedAddress {
		return nil, domainErr.NewWithCode(domainErr.ErrUnauthorized, "public_key_address_mismatch", "public key does not match the challenged wallet", nil)
	}
	signedBytes, err := SignedBytes(challenge.Transport, challenge.Message)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), signedBytes, signature) {
		return nil, domainErr.NewWithCode(domainErr.ErrUnauthorized, "invalid_nimiq_signature", "Nimiq signature verification failed", nil)
	}

	user, err := uc.repo.ConsumeAndResolveIdentity(ctx, challenge.ID, publicKey, now, req.AccountLabel)
	if err != nil {
		return nil, err
	}
	if !user.IsActive() {
		return nil, domainErr.NewWithCode(domainErr.ErrForbidden, "account_unavailable", "this account is no longer available", nil)
	}
	token, err := uc.auth.GenerateToken(ctx, service.TokenClaims{UserID: user.ID, Email: user.Email})
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create authenticated session", err)
	}
	return &dto.LoginResponse{Token: token, User: dto.ToUserInfo(user, challenge.ClaimedAddress)}, nil
}

func (uc *NimiqAuthUseCase) AddressForUser(ctx context.Context, userID uuid.UUID) (string, error) {
	if uc == nil || uc.repo == nil {
		return "", nil
	}
	return uc.repo.AddressForUser(ctx, userID)
}

func (uc *NimiqAuthUseCase) VerifiedAddresses(ctx context.Context, userID uuid.UUID) ([]string, error) {
	if uc == nil || uc.repo == nil {
		return nil, nil
	}
	return uc.repo.VerifiedAddresses(ctx, userID)
}

func (uc *NimiqAuthUseCase) ChallengeClaimedAddress(ctx context.Context, challengeID uuid.UUID) string {
	if uc == nil || uc.repo == nil {
		return ""
	}
	challenge, err := uc.repo.GetChallenge(ctx, challengeID)
	if err != nil || challenge == nil {
		return ""
	}
	return challenge.ClaimedAddress
}

func SanitizeAccountLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 100 {
		return ""
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return value
}

func DisplayNameForWallet(address, accountLabel string) string {
	if label := SanitizeAccountLabel(accountLabel); label != "" {
		return label
	}
	return address
}

func networkMismatchError(code, message, requestedNetwork, requestedEnvironment string, expected nimiqnet.Network) error {
	details := map[string]string{
		"requested_network":     publicNetworkName(requestedNetwork),
		"requested_environment": publicEnvironmentName(requestedEnvironment),
		"expected_network":      expected.AuthName,
		"expected_environment":  expected.Environment,
	}
	return domainErr.NewWithCodeAndDetails(
		domainErr.ErrBadRequest,
		code,
		fmt.Sprintf("%s (requested_network=%s requested_environment=%s expected_network=%s expected_environment=%s)", message, details["requested_network"], details["requested_environment"], details["expected_network"], details["expected_environment"]),
		nil,
		details,
	)
}

func publicNetworkName(value string) string {
	if parsed, err := nimiqnet.ParseNetwork(value); err == nil {
		return parsed.AuthName
	}
	return publicConfigToken(value)
}

func publicEnvironmentName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case nimiqnet.EnvironmentTestnet:
		return nimiqnet.EnvironmentTestnet
	case nimiqnet.EnvironmentMainnet:
		return nimiqnet.EnvironmentMainnet
	default:
		return publicConfigToken(value)
	}
}

func publicConfigToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unset"
	}
	if utf8.RuneCountInString(value) > 32 {
		return "invalid"
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return "invalid"
		}
	}
	return value
}

func SignedBytes(transport, message string) ([]byte, error) {
	switch transport {
	case NimiqTransportMiniApp:
		return []byte(message), nil
	case NimiqTransportHub:
		payload := nimiqSignedMessagePrefix + strconv.Itoa(len(message)) + message
		digest := sha256.Sum256([]byte(payload))
		return digest[:], nil
	default:
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "unsupported_auth_transport", "unsupported Nimiq authentication transport", nil)
	}
}

func decodeFixedHex(value string, size int) ([]byte, error) {
	value = strings.TrimSpace(value)
	if len(value) != size*2 {
		return nil, fmt.Errorf("wrong length")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
func allZero(value []byte) bool {
	for _, b := range value {
		if b != 0 {
			return false
		}
	}
	return true
}

func NormalizeNimiqAddress(value string) (string, error) {
	return nimiqnet.NormalizeUserFriendlyAddress(value)
}

func AddressFromPublicKey(publicKey []byte) string {
	digest := blake2b.Sum256(publicKey)
	payload := nimiqBase32(digest[:20])
	provisional := "NQ00" + payload
	checksum := 98 - ibanMod97(payload+"NQ00")
	compact := fmt.Sprintf("NQ%02d%s", checksum, provisional[4:])
	return formatNimiqAddress(compact)
}

func nimiqBase32(data []byte) string {
	var out strings.Builder
	for bit := 0; bit < len(data)*8; bit += 5 {
		value := 0
		for offset := 0; offset < 5; offset++ {
			index := bit + offset
			value <<= 1
			if index < len(data)*8 && data[index/8]&(1<<uint(7-index%8)) != 0 {
				value |= 1
			}
		}
		out.WriteByte(nimiqAlphabet[value])
	}
	return out.String()
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
func formatNimiqAddress(compact string) string {
	var parts []string
	for i := 0; i < len(compact); i += 4 {
		parts = append(parts, compact[i:i+4])
	}
	return strings.Join(parts, " ")
}
