package usecase

import (
	"context"
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
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

var transactionHashPattern = regexp.MustCompile("^[0-9a-fA-F]{64}$")

// PurchaseUseCase owns the client boundary and delegates blockchain truth to a verifier.
type PurchaseUseCase struct {
	repo         repository.PurchaseRepository
	verifier     verification.Verifier
	now          func() time.Time
	holdDuration time.Duration
	recipient    string
	network      string
}

// NewPurchaseUseCase creates the purchase use case with verification disabled when no verifier is configured.
func NewPurchaseUseCase(repo repository.PurchaseRepository, holdDuration time.Duration) *PurchaseUseCase {
	if holdDuration <= 0 {
		holdDuration = 10 * time.Minute
	}
	return &PurchaseUseCase{repo: repo, holdDuration: holdDuration, now: func() time.Time { return time.Now().UTC() }}
}

// NewPurchaseUseCaseWithVerifier creates the purchase use case for a configured Nimiq verifier.
func NewPurchaseUseCaseWithVerifier(repo repository.PurchaseRepository, holdDuration time.Duration, verifier verification.Verifier, recipient, network string) *PurchaseUseCase {
	uc := NewPurchaseUseCase(repo, holdDuration)
	uc.verifier = verifier
	uc.recipient = recipient
	uc.network = network
	return uc
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
	if strings.TrimSpace(uc.recipient) == "" || strings.TrimSpace(uc.network) == "" {
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
	purchase, err := uc.repo.SubmitTransaction(ctx, purchaseID, userID, normalized, uc.now().UTC())
	if err != nil {
		return nil, err
	}
	purchase = uc.verifyAndPersist(ctx, purchase)
	return &dto.PurchaseResponse{Data: mapPurchase(purchase)}, nil
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

func (uc *PurchaseUseCase) verifyAndPersist(ctx context.Context, purchase *model.Purchase) *model.Purchase {
	if purchase == nil || uc.verifier == nil ||
		(purchase.Status != model.StatusSubmitted && purchase.Status != model.StatusVerifying) {
		return purchase
	}
	outcome, err := uc.verifier.Verify(ctx, purchase)
	if err != nil {
		return purchase
	}
	var next model.Status
	switch outcome {
	case verification.OutcomeNotFound:
		return purchase
	case verification.OutcomeNotFinal:
		next = model.StatusVerifying
	case verification.OutcomeConfirmed:
		next = model.StatusConfirmed
	case verification.OutcomeInvalid:
		next = model.StatusFailed
	default:
		return purchase
	}
	updated, err := uc.repo.SetVerificationState(ctx, purchase.ID, purchase.UserID, next, uc.now().UTC())
	if err != nil || updated == nil {
		return purchase
	}
	return updated
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
