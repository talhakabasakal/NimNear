package usecase

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	eventUC "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/dto"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/repository"
	realtimeevent "github.com/masterfabric-go/masterfabric/internal/domain/realtime/event"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
)

var transactionHashPattern = regexp.MustCompile("^[0-9a-fA-F]{64}$")

const (
	defaultReconciliationInterval  = time.Minute
	defaultReconciliationDeadline  = time.Hour
	defaultReconciliationBatchSize = 50
)

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

// PurchaseUseCase owns the client boundary and delegates blockchain truth to a verifier.
type PurchaseUseCase struct {
	repo           repository.PurchaseRepository
	verifier       verification.Verifier
	identities     IdentityResolver
	now            func() time.Time
	holdDuration   time.Duration
	recipient      string
	network        string
	reconciliation ReconciliationPolicy
	events         events.EventBus
}

// NewPurchaseUseCase creates the purchase use case with verification disabled when no verifier is configured.
func NewPurchaseUseCase(repo repository.PurchaseRepository, holdDuration time.Duration) *PurchaseUseCase {
	return NewPurchaseUseCaseWithPolicy(repo, holdDuration, ReconciliationPolicy{})
}

// NewPurchaseUseCaseWithPolicy creates a purchase use case with bounded reconciliation settings.
func NewPurchaseUseCaseWithPolicy(repo repository.PurchaseRepository, holdDuration time.Duration, policy ReconciliationPolicy) *PurchaseUseCase {
	if holdDuration <= 0 {
		holdDuration = 10 * time.Minute
	}
	return &PurchaseUseCase{
		repo:           repo,
		holdDuration:   holdDuration,
		now:            func() time.Time { return time.Now().UTC() },
		reconciliation: normalizeReconciliationPolicy(policy),
	}
}

// NewPurchaseUseCaseWithVerifier creates the purchase use case for a configured Nimiq verifier.
func NewPurchaseUseCaseWithVerifier(repo repository.PurchaseRepository, holdDuration time.Duration, verifier verification.Verifier, recipient, network string) *PurchaseUseCase {
	return NewPurchaseUseCaseWithVerifierAndPolicy(repo, holdDuration, verifier, recipient, network, ReconciliationPolicy{})
}

// NewPurchaseUseCaseWithVerifierAndPolicy creates a verified purchase use case with explicit reconciliation settings.
func NewPurchaseUseCaseWithVerifierAndPolicy(repo repository.PurchaseRepository, holdDuration time.Duration, verifier verification.Verifier, recipient, network string, policy ReconciliationPolicy) *PurchaseUseCase {
	uc := NewPurchaseUseCaseWithPolicy(repo, holdDuration, policy)
	uc.verifier = verifier
	uc.recipient = recipient
	uc.network = network
	return uc
}

// WithEventBus attaches best-effort realtime publishing after committed transitions.
func (uc *PurchaseUseCase) WithEventBus(bus events.EventBus) *PurchaseUseCase {
	if uc != nil {
		uc.events = bus
	}
	return uc
}

// IdentityResolver returns the authenticated user's verified Nimiq addresses.
type IdentityResolver interface {
	VerifiedAddresses(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// WithIdentities attaches the authoritative Nimiq identity repository used by sender binding.
func (uc *PurchaseUseCase) WithIdentities(identities IdentityResolver) *PurchaseUseCase {
	if uc != nil {
		uc.identities = identities
	}
	return uc
}

func (uc *PurchaseUseCase) paymentsConfigured() bool {
	return uc != nil && uc.verifier != nil && strings.TrimSpace(uc.recipient) != "" && strings.TrimSpace(uc.network) != ""
}

func (uc *PurchaseUseCase) requireVerifiedIdentity(ctx context.Context, userID uuid.UUID) error {
	if uc.identities == nil {
		return domainErr.NewWithCode(domainErr.ErrNotFound, "no_verified_nimiq_identity", "no verified Nimiq identity is linked to this account", nil)
	}
	addresses, err := uc.identities.VerifiedAddresses(ctx, userID)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return domainErr.NewWithCode(domainErr.ErrNotFound, "no_verified_nimiq_identity", "no verified Nimiq identity is linked to this account", nil)
	}
	return nil
}

// Create creates or returns the authenticated user's active purchase for an event.
// The repository derives the amount from the locked event row.
func (uc *PurchaseUseCase) Create(ctx context.Context, eventID, userID uuid.UUID) (*dto.PurchaseResponse, error) {
	if err := validateIDs(eventID, userID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "purchase repository is not configured", nil)
	}
	if !uc.paymentsConfigured() {
		return nil, domainErr.NewWithCode(domainErr.ErrNotImplemented, "payment_not_configured", "Nimiq payment configuration is not enabled", nil)
	}
	if err := uc.requireVerifiedIdentity(ctx, userID); err != nil {
		return nil, err
	}
	purchase, err := uc.repo.Create(ctx, eventID, userID, uc.now().UTC(), uc.holdDuration)
	if err != nil {
		return nil, err
	}
	return &dto.PurchaseResponse{Data: mapPurchase(purchase)}, nil
}

// GetActiveForEvent returns the owner's active purchase for refresh/recovery.
func (uc *PurchaseUseCase) GetActiveForEvent(ctx context.Context, eventID, userID uuid.UUID) (*dto.PurchaseResponse, error) {
	if err := validateIDs(eventID, userID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "purchase repository is not configured", nil)
	}
	purchase, err := uc.repo.GetActiveForEvent(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	purchase = uc.verifyAndPersist(ctx, purchase)
	return &dto.PurchaseResponse{Data: mapPurchase(purchase)}, nil
}

// GetOwned returns a purchase only when it belongs to the authenticated user.
// Submitted/verifying purchases are safely rechecked to support refresh recovery.
func (uc *PurchaseUseCase) GetOwned(ctx context.Context, purchaseID, userID uuid.UUID) (*dto.PurchaseResponse, error) {
	purchase, err := uc.getOwnedAndMaybeVerify(ctx, purchaseID, userID)
	if err != nil {
		return nil, err
	}
	return &dto.PurchaseResponse{Data: mapPurchase(purchase)}, nil
}

// PaymentInstructions returns backend-authoritative values for a pending purchase.
func (uc *PurchaseUseCase) PaymentInstructions(ctx context.Context, purchaseID, userID uuid.UUID) (*dto.PaymentInstructionsResponse, error) {
	if err := validatePurchaseIdentity(purchaseID, userID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(uc.recipient) == "" || strings.TrimSpace(uc.network) == "" || uc.verifier == nil {
		return nil, domainErr.NewWithCode(domainErr.ErrNotImplemented, "payment_not_configured", "Nimiq payment configuration is not enabled", nil)
	}
	purchase, err := uc.repo.GetOwned(ctx, purchaseID, userID)
	if err != nil {
		return nil, err
	}
	if purchase.Status != model.StatusPending {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "purchase_not_pending", "purchase is no longer waiting for wallet payment", nil)
	}
	return &dto.PaymentInstructionsResponse{Data: dto.PaymentInstructions{
		PurchaseID:            purchase.ID,
		Recipient:             uc.recipient,
		AmountLunas:           strconv.FormatInt(purchase.AmountLunas, 10),
		Network:               uc.network,
		CapacityHoldExpiresAt: purchase.CapacityHoldExpiresAt,
	}}, nil
}

// SubmitTransaction persists the wallet hash and performs one safe verification attempt.
func (uc *PurchaseUseCase) SubmitTransaction(ctx context.Context, purchaseID, userID uuid.UUID, transactionHash string) (*dto.PurchaseResponse, error) {
	if err := validatePurchaseIdentity(purchaseID, userID); err != nil {
		return nil, err
	}
	normalized := strings.ToLower(strings.TrimSpace(transactionHash))
	if !transactionHashPattern.MatchString(normalized) {
		return nil, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_transaction_hash", "transaction hash must be 64 hexadecimal characters", nil)
	}
	if !uc.paymentsConfigured() {
		return nil, domainErr.NewWithCode(domainErr.ErrNotImplemented, "payment_not_configured", "Nimiq payment configuration is not enabled", nil)
	}
	if err := uc.requireVerifiedIdentity(ctx, userID); err != nil {
		return nil, err
	}
	now := uc.now().UTC()
	purchase, err := uc.repo.SubmitTransaction(ctx, purchaseID, userID, normalized, now, now.Add(uc.reconciliation.Deadline))
	if err != nil {
		return nil, err
	}
	if purchase.Status == model.StatusSubmitted {
		uc.publishStatus(ctx, purchase)
	}
	purchase = uc.verifyAndPersist(ctx, purchase)
	return &dto.PurchaseResponse{Data: mapPurchase(purchase)}, nil
}

// RecoverExpired re-verifies an expired purchase that already has a reserved hash.
// It may become confirmed only when the same full verifier succeeds for that hash.
func (uc *PurchaseUseCase) RecoverExpired(ctx context.Context, purchaseID, userID uuid.UUID) (*dto.PurchaseResponse, error) {
	if err := validatePurchaseIdentity(purchaseID, userID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "purchase repository is not configured", nil)
	}
	if !uc.paymentsConfigured() {
		return nil, domainErr.NewWithCode(domainErr.ErrNotImplemented, "payment_not_configured", "Nimiq payment configuration is not enabled", nil)
	}
	purchase, err := uc.repo.GetOwned(ctx, purchaseID, userID)
	if err != nil {
		return nil, err
	}
	before := purchase.Status
	if purchase.Status == model.StatusConfirmed {
		slog.Info("payment recovery", "domain", "event_purchase", "purchase_id", purchase.ID, "event_id", purchase.EventID, "previous_status", before, "new_status", purchase.Status, "recovery_result", "already_paid")
		return &dto.PurchaseResponse{Data: mapPurchase(purchase)}, nil
	}
	if purchase.Status != model.StatusExpired || purchase.TransactionHash == nil || strings.TrimSpace(*purchase.TransactionHash) == "" {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "purchase_not_recoverable", "purchase is not an expired payment with a reserved transaction", nil)
	}

	outcome, verifyErr := uc.verifier.Verify(ctx, purchase)
	if verifyErr != nil {
		slog.Info("payment recovery", "domain", "event_purchase", "purchase_id", purchase.ID, "event_id", purchase.EventID, "previous_status", before, "new_status", purchase.Status, "recovery_result", "rpc_transient")
		return &dto.PurchaseResponse{Data: mapPurchase(purchase)}, nil
	}
	if outcome != verification.OutcomeConfirmed {
		result := string(outcome)
		if result == "" {
			result = "unresolved"
		}
		slog.Info("payment recovery", "domain", "event_purchase", "purchase_id", purchase.ID, "event_id", purchase.EventID, "previous_status", before, "new_status", purchase.Status, "recovery_result", result)
		return &dto.PurchaseResponse{Data: mapPurchase(purchase)}, nil
	}

	updated, err := uc.repo.ConfirmExpiredRecovery(ctx, purchase.ID, purchase.UserID, uc.now().UTC())
	if err != nil {
		return nil, err
	}
	if updated.Status != before {
		uc.publishStatus(ctx, updated)
	}
	slog.Info("payment recovery", "domain", "event_purchase", "purchase_id", updated.ID, "event_id", updated.EventID, "previous_status", before, "new_status", updated.Status, "recovery_result", "confirmed")
	return &dto.PurchaseResponse{Data: mapPurchase(updated)}, nil
}

func validateIDs(eventID, userID uuid.UUID) error {
	if eventID == uuid.Nil {
		return domainErr.New(domainErr.ErrBadRequest, "event id is required", nil)
	}
	if userID == uuid.Nil {
		return domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil)
	}
	return nil
}

func validatePurchaseIdentity(purchaseID, userID uuid.UUID) error {
	if purchaseID == uuid.Nil {
		return domainErr.New(domainErr.ErrBadRequest, "purchase id is required", nil)
	}
	if userID == uuid.Nil {
		return domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil)
	}
	return nil
}

func (uc *PurchaseUseCase) getOwnedAndMaybeVerify(ctx context.Context, purchaseID, userID uuid.UUID) (*model.Purchase, error) {
	if err := validatePurchaseIdentity(purchaseID, userID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "purchase repository is not configured", nil)
	}
	purchase, err := uc.repo.GetOwned(ctx, purchaseID, userID)
	if err != nil {
		return nil, err
	}
	if purchase == nil {
		return nil, domainErr.New(domainErr.ErrNotFound, "purchase not found", nil)
	}
	if purchase.Status == model.StatusSubmitted || purchase.Status == model.StatusVerifying {
		purchase = uc.verifyAndPersist(ctx, purchase)
	}
	return purchase, nil
}

type reconciliationClassification string

const (
	classificationConfirmed        reconciliationClassification = "confirmed"
	classificationInvalid          reconciliationClassification = "invalid"
	classificationNotFinal         reconciliationClassification = "not_final"
	classificationNotFound         reconciliationClassification = "not_found_unresolved"
	classificationTransient        reconciliationClassification = "rpc_transient"
	classificationStaleExpired     reconciliationClassification = "stale_unresolved_expired"
	classificationStateUpdateError reconciliationClassification = "state_update_error"
	classificationClaimUnavailable reconciliationClassification = "claim_unavailable"
)

// ReconciliationResult is an operationally safe summary for structured logs.
type ReconciliationResult struct {
	PurchaseID uuid.UUID
	EventID    uuid.UUID
	Before     model.Status
	After      model.Status
	Class      string
}

// ReconciliationReport describes one bounded reconciliation pass.
type ReconciliationReport struct {
	Selected int
	Results  []ReconciliationResult
}

func (uc *PurchaseUseCase) verifyAndPersist(ctx context.Context, purchase *model.Purchase) *model.Purchase {
	updated, _ := uc.reconcilePurchase(ctx, purchase)
	return updated
}

func (uc *PurchaseUseCase) reconcilePurchase(ctx context.Context, purchase *model.Purchase) (*model.Purchase, ReconciliationResult) {
	result := ReconciliationResult{}
	if purchase == nil {
		return purchase, result
	}
	result.PurchaseID = purchase.ID
	result.EventID = purchase.EventID
	result.Before = purchase.Status
	result.After = purchase.Status
	if uc.repo == nil || uc.verifier == nil ||
		(purchase.Status != model.StatusSubmitted && purchase.Status != model.StatusVerifying) {
		return purchase, result
	}

	now := uc.now().UTC()
	stale := uc.reconciliationDeadlineElapsed(purchase, now)
	claimed, err := uc.repo.ClaimVerification(ctx, purchase.ID, purchase.UserID, now, uc.reconciliation.Interval, uc.reconciliation.Interval, stale)
	if err != nil {
		result.Class = string(classificationStateUpdateError)
		return purchase, result
	}
	if claimed == nil {
		result.Class = string(classificationClaimUnavailable)
		return purchase, result
	}

	outcome, verifyErr := uc.verifier.Verify(ctx, claimed)
	if verifyErr != nil {
		if stale {
			return uc.expireStale(ctx, claimed, result, classificationStaleExpired)
		}
		result.Class = string(classificationTransient)
		return purchase, result
	}

	switch outcome {
	case verification.OutcomeConfirmed:
		return uc.persistVerificationState(ctx, claimed, model.StatusConfirmed, result, classificationConfirmed)
	case verification.OutcomeInvalid:
		return uc.persistVerificationState(ctx, claimed, model.StatusFailed, result, classificationInvalid)
	case verification.OutcomeNotFinal:
		if stale {
			return uc.expireStale(ctx, claimed, result, classificationStaleExpired)
		}
		return uc.persistVerificationState(ctx, claimed, model.StatusVerifying, result, classificationNotFinal)
	case verification.OutcomeNotFound:
		if stale {
			return uc.expireStale(ctx, claimed, result, classificationStaleExpired)
		}
		result.Class = string(classificationNotFound)
		return purchase, result
	default:
		if stale {
			return uc.expireStale(ctx, claimed, result, classificationStaleExpired)
		}
		result.Class = string(classificationTransient)
		return purchase, result
	}
}

func (uc *PurchaseUseCase) persistVerificationState(ctx context.Context, purchase *model.Purchase, status model.Status, result ReconciliationResult, class reconciliationClassification) (*model.Purchase, ReconciliationResult) {
	before := purchase.Status
	updated, err := uc.repo.SetVerificationState(ctx, purchase.ID, purchase.UserID, status, uc.now().UTC())
	if err != nil || updated == nil {
		result.Class = string(classificationStateUpdateError)
		return purchase, result
	}
	result.After = updated.Status
	result.Class = string(class)
	if updated.Status != before {
		uc.publishStatus(ctx, updated)
	}
	return updated, result
}

func (uc *PurchaseUseCase) expireStale(ctx context.Context, purchase *model.Purchase, result ReconciliationResult, class reconciliationClassification) (*model.Purchase, ReconciliationResult) {
	before := purchase.Status
	updated, err := uc.repo.ExpireUnresolved(ctx, purchase.ID, purchase.UserID, uc.now().UTC(), uc.reconciliation.Deadline)
	if err != nil || updated == nil {
		result.Class = string(classificationStateUpdateError)
		return purchase, result
	}
	result.After = updated.Status
	result.Class = string(class)
	if updated.Status != before {
		uc.publishStatus(ctx, updated)
	}
	return updated, result
}

func (uc *PurchaseUseCase) reconciliationDeadlineElapsed(purchase *model.Purchase, now time.Time) bool {
	deadline := purchase.UpdatedAt.Add(uc.reconciliation.Deadline)
	if purchase.ReconciliationDeadlineAt != nil {
		deadline = *purchase.ReconciliationDeadlineAt
	}
	return !now.Before(deadline)
}

// Reconcile processes at most the configured batch size and continues after an individual failure.
func (uc *PurchaseUseCase) Reconcile(ctx context.Context) (ReconciliationReport, error) {
	now := uc.now().UTC()
	candidates, err := uc.repo.ListReconciliationCandidates(ctx, now, uc.reconciliation.Interval, uc.reconciliation.Deadline, uc.reconciliation.BatchSize)
	if err != nil {
		return ReconciliationReport{}, err
	}
	report := ReconciliationReport{Selected: len(candidates), Results: make([]ReconciliationResult, 0, len(candidates))}
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		_, result := uc.reconcilePurchase(ctx, candidate)
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func mapPurchase(purchase *model.Purchase) dto.PurchaseInfo {
	if purchase == nil {
		return dto.PurchaseInfo{}
	}
	return dto.PurchaseInfo{
		ID:                    purchase.ID,
		EventID:               purchase.EventID,
		AmountLunas:           strconv.FormatInt(purchase.AmountLunas, 10),
		AmountNIM:             formatNIM(purchase.AmountLunas),
		Status:                string(purchase.Status),
		TransactionHash:       purchase.TransactionHash,
		CapacityHoldExpiresAt: purchase.CapacityHoldExpiresAt,
		ConfirmedAt:           purchase.ConfirmedAt,
		CreatedAt:             purchase.CreatedAt,
		UpdatedAt:             purchase.UpdatedAt,
	}
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

func (uc *PurchaseUseCase) publishStatus(ctx context.Context, purchase *model.Purchase) {
	if uc == nil || uc.events == nil || purchase == nil {
		return
	}
	eventType := realtimeevent.EventPurchaseEventType(string(purchase.Status))
	if eventType == "" {
		return
	}
	event := realtimeevent.StatusChanged{
		Type:       eventType,
		ResourceID: purchase.ID.String(),
		Status:     string(purchase.Status),
		UserIDs:    realtimeevent.UniqueUserIDs(purchase.UserID),
		OccurredAt: uc.now().UTC(),
	}
	_ = uc.events.Publish(ctx, events.TopicPayments, event)
	if purchase.Status == model.StatusConfirmed {
		_ = uc.events.Publish(ctx, events.TopicPayments, realtimeevent.StatusChanged{
			Type:       realtimeevent.TypeWalletActivityChanged,
			ResourceID: purchase.ID.String(),
			Status:     string(purchase.Status),
			UserIDs:    event.UserIDs,
			OccurredAt: event.OccurredAt,
		})
	}
}
