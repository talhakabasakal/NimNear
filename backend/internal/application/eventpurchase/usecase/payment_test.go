package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type scriptedVerifier struct {
	outcome verification.Outcome
	calls   int
}

func (v *scriptedVerifier) Verify(context.Context, *model.Purchase) (verification.Outcome, error) {
	v.calls++
	return v.outcome, nil
}

func TestSubmitTransactionUsesServerStateAndVerifierOutcome(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusPending, CreatedAt: now, UpdatedAt: now}
	verifier := &scriptedVerifier{outcome: verification.OutcomeNotFinal}
	repo := &fakePurchaseRepository{purchase: purchase}
	uc := NewPurchaseUseCaseWithVerifier(repo, time.Minute, verifier, "merchant", "MainAlbatross")
	uc.now = func() time.Time { return now }

	result, err := uc.SubmitTransaction(context.Background(), purchase.ID, purchase.UserID, strings.Repeat("A", 64))
	if err != nil {
		t.Fatalf("SubmitTransaction returned error: %v", err)
	}
	if result.Data.Status != string(model.StatusVerifying) || result.Data.TransactionHash == nil {
		t.Fatalf("unexpected submitted response: %#v", result.Data)
	}
	if *result.Data.TransactionHash != strings.Repeat("a", 64) {
		t.Fatalf("hash was not normalized: %q", *result.Data.TransactionHash)
	}
	if verifier.calls != 1 {
		t.Fatalf("verifier calls = %d, want 1", verifier.calls)
	}
}

func TestSubmitTransactionConfirmsOnlyVerifierConfirmation(t *testing.T) {
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusPending}
	verifier := &scriptedVerifier{outcome: verification.OutcomeConfirmed}
	uc := NewPurchaseUseCaseWithVerifier(&fakePurchaseRepository{purchase: purchase}, time.Minute, verifier, "merchant", "MainAlbatross")

	result, err := uc.SubmitTransaction(context.Background(), purchase.ID, purchase.UserID, strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("SubmitTransaction returned error: %v", err)
	}
	if result.Data.Status != string(model.StatusConfirmed) {
		t.Fatalf("status = %q, want confirmed", result.Data.Status)
	}
}

func TestSubmitTransactionRejectsInvalidHashAndDoesNotAcceptClientPaymentFields(t *testing.T) {
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusPending}
	uc := NewPurchaseUseCase(&fakePurchaseRepository{purchase: purchase}, time.Minute)

	result, err := uc.SubmitTransaction(context.Background(), purchase.ID, purchase.UserID, "not-a-hash")
	if err == nil || !errors.Is(err, domainErr.ErrValidation) {
		t.Fatalf("error = %v, want validation error", err)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}
