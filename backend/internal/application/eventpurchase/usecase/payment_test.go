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
	outcome  verification.Outcome
	err      error
	outcomes []verification.Outcome
	calls    int
}

func (v *scriptedVerifier) Verify(context.Context, *model.Purchase) (verification.Outcome, error) {
	v.calls++
	if v.err != nil {
		return v.outcome, v.err
	}
	if len(v.outcomes) > 0 {
		index := v.calls - 1
		if index >= len(v.outcomes) {
			index = len(v.outcomes) - 1
		}
		return v.outcomes[index], nil
	}
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

func TestVerificationTransientErrorRemainsUnresolvedBeforeDeadline(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusSubmitted, CreatedAt: now, UpdatedAt: now}
	verifier := &scriptedVerifier{err: errors.New("rpc unavailable")}
	repo := &fakePurchaseRepository{purchase: purchase}
	uc := NewPurchaseUseCaseWithVerifierAndPolicy(repo, time.Minute, verifier, "merchant", "MainAlbatross", ReconciliationPolicy{Interval: time.Minute, Deadline: time.Hour, BatchSize: 2})
	uc.now = func() time.Time { return now }

	result, err := uc.GetOwned(context.Background(), purchase.ID, purchase.UserID)
	if err != nil {
		t.Fatalf("GetOwned returned error: %v", err)
	}
	if result.Data.Status != string(model.StatusSubmitted) {
		t.Fatalf("status = %q, want submitted", result.Data.Status)
	}
	if verifier.calls != 1 || repo.claimCalls != 1 {
		t.Fatalf("calls = verifier %d, claims %d; want one each", verifier.calls, repo.claimCalls)
	}
}

func TestStaleUnresolvedPaymentExpiresAfterFinalAttempt(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusVerifying, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)}
	verifier := &scriptedVerifier{err: errors.New("rpc unavailable")}
	repo := &fakePurchaseRepository{purchase: purchase}
	uc := NewPurchaseUseCaseWithVerifierAndPolicy(repo, time.Minute, verifier, "merchant", "MainAlbatross", ReconciliationPolicy{Interval: time.Minute, Deadline: time.Hour, BatchSize: 2})
	uc.now = func() time.Time { return now }

	result, err := uc.GetOwned(context.Background(), purchase.ID, purchase.UserID)
	if err != nil {
		t.Fatalf("GetOwned returned error: %v", err)
	}
	if result.Data.Status != string(model.StatusExpired) {
		t.Fatalf("status = %q, want expired", result.Data.Status)
	}
	if verifier.calls != 1 {
		t.Fatalf("verifier calls = %d, want one authoritative final attempt", verifier.calls)
	}
}

func TestStaleFinalAttemptCanConfirmOrFailButNeverConfirmsUncertainPayment(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome verification.Outcome
		want    model.Status
	}{
		{name: "confirmed", outcome: verification.OutcomeConfirmed, want: model.StatusConfirmed},
		{name: "invalid", outcome: verification.OutcomeInvalid, want: model.StatusFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
			purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusSubmitted, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)}
			repo := &fakePurchaseRepository{purchase: purchase}
			uc := NewPurchaseUseCaseWithVerifierAndPolicy(repo, time.Minute, &scriptedVerifier{outcome: test.outcome}, "merchant", "MainAlbatross", ReconciliationPolicy{Interval: time.Minute, Deadline: time.Hour, BatchSize: 2})
			uc.now = func() time.Time { return now }

			result, err := uc.GetOwned(context.Background(), purchase.ID, purchase.UserID)
			if err != nil {
				t.Fatalf("GetOwned returned error: %v", err)
			}
			if result.Data.Status != string(test.want) {
				t.Fatalf("status = %q, want %q", result.Data.Status, test.want)
			}
		})
	}
}

func TestReconcileUsesBoundedBatchAndDeterministicCandidates(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	candidates := make([]*model.Purchase, 3)
	for i := range candidates {
		candidates[i] = &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusSubmitted, CreatedAt: now, UpdatedAt: now}
	}
	repo := &fakePurchaseRepository{candidates: candidates}
	verifier := &scriptedVerifier{outcomes: []verification.Outcome{verification.OutcomeConfirmed, verification.OutcomeInvalid}}
	uc := NewPurchaseUseCaseWithVerifierAndPolicy(repo, time.Minute, verifier, "merchant", "MainAlbatross", ReconciliationPolicy{Interval: time.Minute, Deadline: time.Hour, BatchSize: 2})
	uc.now = func() time.Time { return now }

	report, err := uc.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	if report.Selected != 2 || len(report.Results) != 2 || repo.gotListLimit != 2 {
		t.Fatalf("report = %#v, list limit = %d; want bounded two-item pass", report, repo.gotListLimit)
	}
	if candidates[0].Status != model.StatusConfirmed || candidates[1].Status != model.StatusFailed {
		t.Fatalf("candidate states = %q, %q", candidates[0].Status, candidates[1].Status)
	}
}
