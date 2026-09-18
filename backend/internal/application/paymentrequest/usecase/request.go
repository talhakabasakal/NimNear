package usecase

import (
	"context"
	"log/slog"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	eventUC "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	iamUC "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/application/nimiqtx/verification"
	"github.com/masterfabric-go/masterfabric/internal/application/paymentrequest/dto"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/repository"
	realtimeevent "github.com/masterfabric-go/masterfabric/internal/domain/realtime/event"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
)

var (
	transactionHashPattern = regexp.MustCompile("^[0-9a-fA-F]{64}$")
	amountPattern          = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
)

const (
	// DefaultTTL is the single payment-request lifetime. Public and creator reads
	// treat now >= expires_at as expired even if the row is still pending.
	DefaultTTL = 24 * time.Hour

	MaxNoteLength                  = 140
	DefaultListLimit               = 20
	MaxListLimit                   = 100
	defaultReconciliationInterval  = time.Minute
	defaultReconciliationDeadline  = time.Hour
	defaultReconciliationBatchSize = 50
)

// IdentityResolver returns the authenticated user's verified Nimiq addresses, most recent first.
type IdentityResolver interface {
	VerifiedAddresses(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// ReconciliationPolicy bounds retries and gives unresolved payments an explicit terminal policy.
type ReconciliationPolicy struct {
	Interval  time.Duration
	Deadline  time.Duration
	BatchSize int
}

func normalizeReconciliationPolicy(policy ReconciliationPolicy) ReconciliationPolicy {
	if policy.Interval <= 0 {
		policy.Interval = defaultReconciliationInterval
	}
	if policy.Deadline <= 0 {
		policy.Deadline = defaultReconciliationDeadline
	}
	if policy.BatchSize <= 0 {
		policy.BatchSize = defaultReconciliationBatchSize
	}
	return policy
}

// RequestUseCase owns payment-request creation, reads, cancellation, and verified payment.
type RequestUseCase struct {
	repo           repository.RequestRepository
	identities     IdentityResolver
	verifier       verification.TransferVerifier
	now            func() time.Time
	ttl            time.Duration
	network        string
	logger         *slog.Logger
	reconciliation ReconciliationPolicy
	events         events.EventBus
}

// NewRequestUseCase creates a payment-request use case with verification disabled when no verifier is configured.
func NewRequestUseCase(repo repository.RequestRepository, identities IdentityResolver, ttl time.Duration, network string, logger *slog.Logger) *RequestUseCase {
	return NewRequestUseCaseWithPolicy(repo, identities, ttl, network, logger, ReconciliationPolicy{})
}

// NewRequestUseCaseWithPolicy creates a payment-request use case with bounded reconciliation settings.
func NewRequestUseCaseWithPolicy(repo repository.RequestRepository, identities IdentityResolver, ttl time.Duration, network string, logger *slog.Logger, policy ReconciliationPolicy) *RequestUseCase {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &RequestUseCase{
		repo:           repo,
		identities:     identities,
		now:            func() time.Time { return time.Now().UTC() },
		ttl:            ttl,
		network:        strings.TrimSpace(network),
		logger:         logger,
		reconciliation: normalizeReconciliationPolicy(policy),
	}
}

// NewRequestUseCaseWithVerifier creates a verified payment-request use case.
func NewRequestUseCaseWithVerifier(repo repository.RequestRepository, identities IdentityResolver, ttl time.Duration, network string, logger *slog.Logger, verifier verification.TransferVerifier) *RequestUseCase {
	return NewRequestUseCaseWithVerifierAndPolicy(repo, identities, ttl, network, logger, verifier, ReconciliationPolicy{})
}

// NewRequestUseCaseWithVerifierAndPolicy creates a verified payment-request use case with explicit reconciliation settings.
func NewRequestUseCaseWithVerifierAndPolicy(repo repository.RequestRepository, identities IdentityResolver, ttl time.Duration, network string, logger *slog.Logger, verifier verification.TransferVerifier, policy ReconciliationPolicy) *RequestUseCase {
	uc := NewRequestUseCaseWithPolicy(repo, identities, ttl, network, logger, policy)
	uc.verifier = verifier
	return uc
}

// WithEventBus attaches best-effort realtime publishing after committed transitions.
func (uc *RequestUseCase) WithEventBus(bus events.EventBus) *RequestUseCase {
	if uc != nil {
		uc.events = bus
	}
	return uc
}

// Create derives the recipient from the creator's verified Nimiq identity and stores amount as Luna.
func (uc *RequestUseCase) Create(ctx context.Context, creatorID uuid.UUID, input dto.CreateRequest) (*dto.RequestResponse, error) {
	if creatorID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil)
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "payment request repository is not configured", nil)
	}
	recipient, err := uc.resolveOwnedIdentity(ctx, creatorID, input.Address)
	if err != nil {
		return nil, err
	}
	amountLunas, err := parseAmountLunas(input.AmountNIM)
	if err != nil {
		return nil, err
	}
	note, err := normalizeNote(input.Note)
	if err != nil {
		return nil, err
	}
	now := uc.now().UTC()
	request := &model.Request{
		ID:               uuid.New(),
		PublicID:         uuid.New(),
		CreatorUserID:    creatorID,
		RecipientAddress: recipient,
		AmountLunas:      amountLunas,
		Note:             note,
		Status:           model.StatusPending,
		ExpiresAt:        now.Add(uc.ttl),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	created, err := uc.repo.Create(ctx, request)
	if err != nil {
		return nil, err
	}
	uc.log("payment_request_created", "public_id", created.PublicID, "amount_lunas", created.AmountLunas, "status", created.Status)
	return &dto.RequestResponse{Data: uc.mapCreator(created)}, nil
}

// ListOwned returns the creator's payment requests, applying lazy expiry.
func (uc *RequestUseCase) ListOwned(ctx context.Context, creatorID uuid.UUID, limit int) (*dto.RequestsResponse, error) {
	if creatorID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil)
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "payment request repository is not configured", nil)
	}
	if limit == 0 {
		limit = DefaultListLimit
	}
	if limit < 1 || limit > MaxListLimit {
		return nil, domainErr.New(domainErr.ErrBadRequest, "limit must be between 1 and 100", nil)
	}
	requests, err := uc.repo.ListByCreator(ctx, creatorID, uc.now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	items := make([]dto.RequestInfo, 0, len(requests))
	for _, request := range requests {
		items = append(items, uc.mapCreator(uc.maybeVerify(ctx, request)))
	}
	return &dto.RequestsResponse{Data: items}, nil
}

// GetOwned returns a creator-scoped payment request.
func (uc *RequestUseCase) GetOwned(ctx context.Context, publicID, creatorID uuid.UUID) (*dto.RequestResponse, error) {
	request, err := uc.getOwned(ctx, publicID, creatorID)
	if err != nil {
		return nil, err
	}
	return &dto.RequestResponse{Data: uc.mapCreator(request)}, nil
}

// GetPublic returns only the payer-needed fields for a shareable payment request.
func (uc *RequestUseCase) GetPublic(ctx context.Context, publicID uuid.UUID) (*dto.PublicRequestResponse, error) {
	if publicID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrBadRequest, "payment request id is required", nil)
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "payment request repository is not configured", nil)
	}
	request, err := uc.repo.GetByPublicID(ctx, publicID, uc.now().UTC())
	if err != nil {
		return nil, err
	}
	request = uc.maybeVerify(ctx, request)
	return &dto.PublicRequestResponse{Data: uc.mapPublic(request)}, nil
}

// Cancel lets the creator cancel a pending request. Already-cancelled and already-expired
// reads are idempotent; paid and in-flight payments are rejected.
func (uc *RequestUseCase) Cancel(ctx context.Context, publicID, creatorID uuid.UUID) (*dto.RequestResponse, error) {
	if err := validateRequestIdentity(publicID, creatorID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "payment request repository is not configured", nil)
	}
	now := uc.now().UTC()
	before, err := uc.repo.GetOwnedByPublicID(ctx, publicID, creatorID, now)
	if err != nil {
		return nil, err
	}
	request, err := uc.repo.Cancel(ctx, publicID, creatorID, now)
	if err != nil {
		return nil, err
	}
	if before != nil && before.Status != request.Status {
		uc.publishStatus(ctx, request)
	}
	if request.Status == model.StatusCancelled {
		uc.log("payment_request_cancelled", "public_id", request.PublicID, "status", request.Status)
	}
	return &dto.RequestResponse{Data: uc.mapCreator(request)}, nil
}

// SubmitTransaction persists only the hash from an authenticated payer and verifies it against RPC.
func (uc *RequestUseCase) SubmitTransaction(ctx context.Context, publicID, payerID uuid.UUID, transactionHash string) (*dto.RequestResponse, error) {
	if err := validateRequestIdentity(publicID, payerID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "payment request repository is not configured", nil)
	}
	if uc.verifier == nil || strings.TrimSpace(uc.network) == "" {
		return nil, domainErr.NewWithCode(domainErr.ErrNotImplemented, "payment_not_configured", "Nimiq payment configuration is not enabled", nil)
	}
	addresses, err := uc.verifiedAddresses(ctx, payerID)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, domainErr.NewWithCode(domainErr.ErrNotFound, "no_verified_nimiq_identity", "no verified Nimiq identity is linked to this account", nil)
	}
	normalized := strings.ToLower(strings.TrimSpace(transactionHash))
	if !transactionHashPattern.MatchString(normalized) {
		return nil, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_transaction_hash", "transaction hash must be 64 hexadecimal characters", nil)
	}
	now := uc.now().UTC()
	request, err := uc.repo.SubmitTransaction(ctx, publicID, payerID, normalized, now, now.Add(uc.reconciliation.Deadline))
	if err != nil {
		return nil, err
	}
	if request.Status == model.StatusSubmitted {
		uc.publishStatus(ctx, request)
	}
	uc.log("payment_request_transaction_submitted", "public_id", request.PublicID, "status", request.Status)
	request = uc.verifyAndPersist(ctx, request)
	return &dto.RequestResponse{Data: uc.mapCreator(request)}, nil
}

// RecoverExpired re-verifies an expired payment request that already has a reserved hash.
func (uc *RequestUseCase) RecoverExpired(ctx context.Context, publicID, actorID uuid.UUID) (*dto.RequestResponse, error) {
	if err := validateRequestIdentity(publicID, actorID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "payment request repository is not configured", nil)
	}
	if uc.verifier == nil || strings.TrimSpace(uc.network) == "" {
		return nil, domainErr.NewWithCode(domainErr.ErrNotImplemented, "payment_not_configured", "Nimiq payment configuration is not enabled", nil)
	}
	request, err := uc.repo.GetByPublicID(ctx, publicID, uc.now().UTC())
	if err != nil {
		return nil, err
	}
	if request.CreatorUserID != actorID && (request.PayerUserID == nil || *request.PayerUserID != actorID) {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	before := request.Status
	if request.Status == model.StatusPaid {
		uc.log("payment_recovery", "domain", "payment_request", "public_id", request.PublicID, "previous_status", before, "new_status", request.Status, "recovery_result", "already_paid")
		return &dto.RequestResponse{Data: uc.mapCreator(request)}, nil
	}
	if request.Status != model.StatusExpired || request.TransactionHash == nil || strings.TrimSpace(*request.TransactionHash) == "" {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_recoverable", "payment request is not an expired payment with a reserved transaction", nil)
	}

	payerID := actorID
	if request.PayerUserID != nil {
		payerID = *request.PayerUserID
	}
	outcome, verifyErr := uc.verifier.VerifyTransfer(ctx, verification.Transfer{
		TransactionHash: *request.TransactionHash,
		Recipient:       request.RecipientAddress,
		AmountLunas:     request.AmountLunas,
		PayerUserID:     payerID,
	})
	if verifyErr != nil {
		uc.log("payment_recovery", "domain", "payment_request", "public_id", request.PublicID, "previous_status", before, "new_status", request.Status, "recovery_result", "rpc_transient")
		return &dto.RequestResponse{Data: uc.mapCreator(request)}, nil
	}
	if outcome != verification.OutcomeConfirmed {
		uc.log("payment_recovery", "domain", "payment_request", "public_id", request.PublicID, "previous_status", before, "new_status", request.Status, "recovery_result", string(outcome))
		return &dto.RequestResponse{Data: uc.mapCreator(request)}, nil
	}

	updated, err := uc.repo.ConfirmExpiredRecovery(ctx, request.ID, uc.now().UTC())
	if err != nil {
		return nil, err
	}
	if updated.Status != before {
		uc.publishStatus(ctx, updated)
	}
	uc.log("payment_recovery", "domain", "payment_request", "public_id", updated.PublicID, "previous_status", before, "new_status", updated.Status, "recovery_result", "paid")
	return &dto.RequestResponse{Data: uc.mapCreator(updated)}, nil
}

func (uc *RequestUseCase) getOwned(ctx context.Context, publicID, creatorID uuid.UUID) (*model.Request, error) {
	if err := validateRequestIdentity(publicID, creatorID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "payment request repository is not configured", nil)
	}
	request, err := uc.repo.GetOwnedByPublicID(ctx, publicID, creatorID, uc.now().UTC())
	if err != nil {
		return nil, err
	}
	return uc.maybeVerify(ctx, request), nil
}

func (uc *RequestUseCase) maybeVerify(ctx context.Context, request *model.Request) *model.Request {
	if request == nil {
		return nil
	}
	if request.Status == model.StatusSubmitted || request.Status == model.StatusVerifying {
		return uc.verifyAndPersist(ctx, request)
	}
	return request
}

func (uc *RequestUseCase) resolveOwnedIdentity(ctx context.Context, userID uuid.UUID, requested string) (string, error) {
	addresses, err := uc.verifiedAddresses(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(addresses) == 0 {
		return "", domainErr.NewWithCode(domainErr.ErrNotFound, "no_verified_nimiq_identity", "no verified Nimiq identity is linked to this account", nil)
	}
	if strings.TrimSpace(requested) == "" {
		return addresses[0], nil
	}
	normalized, err := iamUC.NormalizeNimiqAddress(requested)
	if err != nil {
		return "", domainErr.NewWithCode(domainErr.ErrBadRequest, "invalid_wallet_address", "invalid Nimiq wallet address", nil)
	}
	compactRequested := compactAddress(normalized)
	for _, address := range addresses {
		if compactAddress(address) == compactRequested {
			return address, nil
		}
	}
	return "", domainErr.NewWithCode(domainErr.ErrForbidden, "wallet_identity_forbidden", "wallet address is not a verified identity of this user", nil)
}

func (uc *RequestUseCase) verifiedAddresses(ctx context.Context, userID uuid.UUID) ([]string, error) {
	if uc.identities == nil {
		return nil, domainErr.NewWithCode(domainErr.ErrNotFound, "no_verified_nimiq_identity", "no verified Nimiq identity is linked to this account", nil)
	}
	return uc.identities.VerifiedAddresses(ctx, userID)
}

func validateRequestIdentity(publicID, userID uuid.UUID) error {
	if publicID == uuid.Nil {
		return domainErr.New(domainErr.ErrBadRequest, "payment request id is required", nil)
	}
	if userID == uuid.Nil {
		return domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil)
	}
	return nil
}

func parseAmountLunas(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_amount", "amount_nim is required", nil)
	}
	if !amountPattern.MatchString(raw) {
		return 0, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_amount", "amount_nim must be a positive decimal with no more than 5 decimal places; no rounding is applied", nil)
	}
	parts := strings.SplitN(raw, ".", 2)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 5 {
		return 0, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_amount", "amount_nim must be a positive decimal with no more than 5 decimal places; no rounding is applied", nil)
	}
	integerPart := new(big.Int)
	if _, ok := integerPart.SetString(parts[0], 10); !ok {
		return 0, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_amount", "amount_nim must be a positive decimal with no more than 5 decimal places; no rounding is applied", nil)
	}
	fraction = fraction + strings.Repeat("0", 5-len(fraction))
	fractionPart := new(big.Int)
	if fraction != "" {
		if _, ok := fractionPart.SetString(fraction, 10); !ok {
			return 0, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_amount", "amount_nim must be a positive decimal with no more than 5 decimal places; no rounding is applied", nil)
		}
	}
	total := new(big.Int).Mul(integerPart, big.NewInt(eventUC.LunasPerNIM))
	total.Add(total, fractionPart)
	if total.Sign() <= 0 {
		return 0, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_amount", "amount_nim must be greater than zero", nil)
	}
	maxInt64 := new(big.Int).SetInt64(int64(^uint64(0) >> 1))
	if total.Cmp(maxInt64) > 0 {
		return 0, domainErr.NewWithCode(domainErr.ErrValidation, "amount_too_large", "amount_nim is too large", nil)
	}
	return total.Int64(), nil
}

func normalizeNote(raw string) (*string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(value) > MaxNoteLength {
		return nil, domainErr.NewWithCode(domainErr.ErrValidation, "note_too_long", "note must be at most 140 characters", nil)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return nil, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_note", "note must be plain text", nil)
		}
	}
	return &value, nil
}

func compactAddress(address string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(address), " ", ""))
}

func formatNIM(lunas int64) string {
	if lunas == 0 {
		return "0"
	}
	whole := lunas / eventUC.LunasPerNIM
	fraction := lunas % eventUC.LunasPerNIM
	if fraction == 0 {
		return strconv.FormatInt(whole, 10)
	}
	text := strconv.FormatInt(fraction+eventUC.LunasPerNIM, 10)[1:]
	for len(text) > 0 && text[len(text)-1] == '0' {
		text = text[:len(text)-1]
	}
	return strconv.FormatInt(whole, 10) + "." + text
}

func (uc *RequestUseCase) mapCreator(request *model.Request) dto.RequestInfo {
	if request == nil {
		return dto.RequestInfo{}
	}
	return dto.RequestInfo{
		PublicID:        request.PublicID,
		Recipient:       request.RecipientAddress,
		AmountLunas:     strconv.FormatInt(request.AmountLunas, 10),
		AmountNIM:       formatNIM(request.AmountLunas),
		Note:            request.Note,
		Status:          string(request.EffectiveStatus(uc.now().UTC())),
		ExpiresAt:       request.ExpiresAt,
		CancelledAt:     request.CancelledAt,
		PaidAt:          request.PaidAt,
		TransactionHash: request.TransactionHash,
		Network:         uc.network,
		CreatedAt:       request.CreatedAt,
		UpdatedAt:       request.UpdatedAt,
	}
}

func (uc *RequestUseCase) mapPublic(request *model.Request) dto.PublicRequestInfo {
	if request == nil {
		return dto.PublicRequestInfo{}
	}
	return dto.PublicRequestInfo{
		PublicID:    request.PublicID,
		Recipient:   request.RecipientAddress,
		AmountLunas: strconv.FormatInt(request.AmountLunas, 10),
		AmountNIM:   formatNIM(request.AmountLunas),
		Note:        request.Note,
		Status:      string(request.EffectiveStatus(uc.now().UTC())),
		ExpiresAt:   request.ExpiresAt,
		Network:     uc.network,
	}
}

func (uc *RequestUseCase) log(event string, args ...any) {
	if uc == nil || uc.logger == nil {
		return
	}
	uc.logger.Info(event, args...)
}

func (uc *RequestUseCase) publishStatus(ctx context.Context, request *model.Request) {
	if uc == nil || uc.events == nil || request == nil {
		return
	}
	eventType := realtimeevent.PaymentRequestEventType(string(request.Status))
	if eventType == "" {
		return
	}
	users := []uuid.UUID{request.CreatorUserID}
	if request.PayerUserID != nil {
		users = append(users, *request.PayerUserID)
	}
	event := realtimeevent.StatusChanged{
		Type:       eventType,
		ResourceID: request.PublicID.String(),
		Status:     string(request.Status),
		UserIDs:    realtimeevent.UniqueUserIDs(users...),
		OccurredAt: uc.now().UTC(),
	}
	if err := uc.events.Publish(ctx, events.TopicPayments, event); err != nil {
		uc.log("domain_event_publish_failed", "type", eventType, "resource_id", request.PublicID, "error", err)
		return
	}
	uc.log("domain_event_published", "type", eventType, "resource_id", request.PublicID, "status", request.Status)
	if request.Status == model.StatusPaid {
		wallet := realtimeevent.StatusChanged{
			Type:       realtimeevent.TypeWalletActivityChanged,
			ResourceID: request.PublicID.String(),
			Status:     string(request.Status),
			UserIDs:    event.UserIDs,
			OccurredAt: event.OccurredAt,
		}
		if err := uc.events.Publish(ctx, events.TopicPayments, wallet); err != nil {
			uc.log("domain_event_publish_failed", "type", wallet.Type, "resource_id", request.PublicID, "error", err)
		}
	}
}
