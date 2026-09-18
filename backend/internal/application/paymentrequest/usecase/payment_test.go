package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/nimiqtx/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type scriptedTransferVerifier struct {
	outcome  verification.Outcome
	err      error
	outcomes []verification.Outcome
	calls    int
	last     verification.Transfer
}

func (v *scriptedTransferVerifier) VerifyTransfer(_ context.Context, transfer verification.Transfer) (verification.Outcome, error) {
	v.calls++
	v.last = transfer
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

func TestSubmitTransactionRequiresAuthenticatedPayerAndVerifier(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	uc := NewRequestUseCase(&fakeRequestRepository{request: request}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil)
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), strings.Repeat("a", 64)); err == nil || domainErr.ErrorCode(err) != "payment_not_configured" {
		t.Fatalf("unconfigured error = %v", err)
	}
	uc = NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: request}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, &scriptedTransferVerifier{outcome: verification.OutcomeConfirmed})
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.Nil, strings.Repeat("a", 64)); !errors.Is(err, domainErr.ErrUnauthorized) {
		t.Fatalf("unauthenticated error = %v", err)
	}
}

func TestSubmitTransactionConfirmsOnlyAfterVerifierAndBindsPayerNotCreator(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	creatorID := uuid.New()
	payerID := uuid.New()
	request := pendingRequest(creatorID, now)
	verifier := &scriptedTransferVerifier{outcome: verification.OutcomeConfirmed}
	repo := &fakeRequestRepository{request: request}
	uc := NewRequestUseCaseWithVerifier(repo, identityStub{addresses: []string{secondIdentity}}, DefaultTTL, "test-albatross", nil, verifier)
	uc.now = func() time.Time { return now }

	result, err := uc.SubmitTransaction(context.Background(), request.PublicID, payerID, strings.Repeat("A", 64))
	if err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	if result.Data.Status != string(model.StatusPaid) {
		t.Fatalf("status = %q", result.Data.Status)
	}
	if verifier.last.PayerUserID != payerID {
		t.Fatalf("payer bound = %s, want %s", verifier.last.PayerUserID, payerID)
	}
	if verifier.last.PayerUserID == creatorID {
		t.Fatal("sender was bound to the request creator")
	}
	if verifier.last.Recipient != primaryIdentity || verifier.last.AmountLunas != 2500000 {
		t.Fatalf("transfer = %#v", verifier.last)
	}
}

func TestSubmitTransactionLeavesSubmittedWhenNotFinal(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	verifier := &scriptedTransferVerifier{outcome: verification.OutcomeNotFinal}
	uc := NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: request}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, verifier)
	uc.now = func() time.Time { return now }
	result, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	if result.Data.Status != string(model.StatusVerifying) {
		t.Fatalf("status = %q, want verifying", result.Data.Status)
	}
}

func TestSubmitTransactionRejectsInvalidHashAndPayerWithoutIdentity(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	uc := NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: request}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, &scriptedTransferVerifier{outcome: verification.OutcomeConfirmed})
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), "not-a-hash"); err == nil || !errors.Is(err, domainErr.ErrValidation) {
		t.Fatalf("invalid hash error = %v", err)
	}
	uc = NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: request}, identityStub{}, DefaultTTL, "test-albatross", nil, &scriptedTransferVerifier{outcome: verification.OutcomeConfirmed})
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), strings.Repeat("a", 64)); err == nil || domainErr.ErrorCode(err) != "no_verified_nimiq_identity" {
		t.Fatalf("missing payer identity error = %v", err)
	}
}

func TestSubmitTransactionRejectsCancelledExpiredPaidAndReplayedHash(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	hash := strings.Repeat("a", 64)
	payer := uuid.New()
	verifier := &scriptedTransferVerifier{outcome: verification.OutcomeConfirmed}

	cancelled := pendingRequest(uuid.New(), now)
	cancelled.Status = model.StatusCancelled
	uc := NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: cancelled}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, verifier)
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), cancelled.PublicID, payer, hash); err == nil || domainErr.ErrorCode(err) != "payment_request_cancelled" {
		t.Fatalf("cancelled error = %v", err)
	}

	expired := pendingRequest(uuid.New(), now.Add(-DefaultTTL))
	uc = NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: expired}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, verifier)
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), expired.PublicID, payer, hash); err == nil || domainErr.ErrorCode(err) != "payment_request_expired" {
		t.Fatalf("expired error = %v", err)
	}

	paid := pendingRequest(uuid.New(), now)
	paid.Status = model.StatusPaid
	uc = NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: paid}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, verifier)
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), paid.PublicID, payer, strings.Repeat("b", 64)); err == nil || domainErr.ErrorCode(err) != "payment_request_already_paid" {
		t.Fatalf("already paid error = %v", err)
	}

	request := pendingRequest(uuid.New(), now)
	uc = NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: request, consumed: map[string]string{hash: uuid.New().String()}}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, verifier)
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, payer, hash); err == nil || domainErr.ErrorCode(err) != "transaction_hash_used" {
		t.Fatalf("replay error = %v", err)
	}
}

func TestVerifierOutcomesFailInvalidAndRetryTransient(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	uc := NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: request}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, &scriptedTransferVerifier{outcome: verification.OutcomeInvalid})
	uc.now = func() time.Time { return now }
	result, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("invalid outcome: %v", err)
	}
	if result.Data.Status != string(model.StatusFailed) {
		t.Fatalf("status = %q, want failed", result.Data.Status)
	}

	retry := pendingRequest(uuid.New(), now)
	retry.Status = model.StatusSubmitted
	hash := strings.Repeat("c", 64)
	retry.TransactionHash = &hash
	payer := uuid.New()
	retry.PayerUserID = &payer
	uc = NewRequestUseCaseWithVerifierAndPolicy(&fakeRequestRepository{request: retry}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, &scriptedTransferVerifier{err: errors.New("rpc unavailable")}, ReconciliationPolicy{Interval: time.Minute, Deadline: time.Hour, BatchSize: 2})
	uc.now = func() time.Time { return now }
	got, err := uc.GetPublic(context.Background(), retry.PublicID)
	if err != nil {
		t.Fatalf("GetPublic: %v", err)
	}
	if got.Data.Status != string(model.StatusSubmitted) {
		t.Fatalf("transient status = %q, want submitted", got.Data.Status)
	}
}

func TestStaleUnresolvedPaymentRequestExpiresAfterFinalAttempt(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now.Add(-2*time.Hour))
	request.Status = model.StatusVerifying
	hash := strings.Repeat("d", 64)
	request.TransactionHash = &hash
	payer := uuid.New()
	request.PayerUserID = &payer
	verifier := &scriptedTransferVerifier{err: errors.New("rpc unavailable")}
	uc := NewRequestUseCaseWithVerifierAndPolicy(&fakeRequestRepository{request: request}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, verifier, ReconciliationPolicy{Interval: time.Minute, Deadline: time.Hour, BatchSize: 2})
	uc.now = func() time.Time { return now }
	got, err := uc.GetPublic(context.Background(), request.PublicID)
	if err != nil {
		t.Fatalf("GetPublic: %v", err)
	}
	if got.Data.Status != string(model.StatusExpired) {
		t.Fatalf("status = %q, want expired", got.Data.Status)
	}
}

func TestReconcileUsesBoundedBatch(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	candidates := make([]*model.Request, 3)
	for i := range candidates {
		payer := uuid.New()
		hash := strings.Repeat("e", 64)
		candidates[i] = pendingRequest(uuid.New(), now)
		candidates[i].Status = model.StatusSubmitted
		candidates[i].PayerUserID = &payer
		candidates[i].TransactionHash = &hash
	}
	repo := &fakeRequestRepository{candidates: candidates}
	verifier := &scriptedTransferVerifier{outcomes: []verification.Outcome{verification.OutcomeConfirmed, verification.OutcomeInvalid}}
	uc := NewRequestUseCaseWithVerifierAndPolicy(repo, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, verifier, ReconciliationPolicy{Interval: time.Minute, Deadline: time.Hour, BatchSize: 2})
	uc.now = func() time.Time { return now }
	report, err := uc.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if report.Selected != 2 || len(report.Results) != 2 || repo.gotListLimit != 2 {
		t.Fatalf("report = %#v limit=%d", report, repo.gotListLimit)
	}
	if candidates[0].Status != model.StatusPaid || candidates[1].Status != model.StatusFailed {
		t.Fatalf("candidate states = %q, %q", candidates[0].Status, candidates[1].Status)
	}
}

func TestConcurrentSubmitAllowsOnlyOnePaidTransition(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	repo := &fakeRequestRepository{request: request}
	uc := NewRequestUseCaseWithVerifier(repo, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, &scriptedTransferVerifier{outcome: verification.OutcomeConfirmed})
	uc.now = func() time.Time { return now }

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			hash := strings.Repeat("f", 63) + string(rune('0'+i))
			_, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), hash)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if domainErr.ErrorCode(err) == "payment_request_not_submittable" || domainErr.ErrorCode(err) == "payment_request_already_paid" {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent error: %v", err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestCrossDomainConsumedHashCannotSatisfyPaymentRequest(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	hash := strings.Repeat("a", 64)
	request := pendingRequest(uuid.New(), now)
	repo := &fakeRequestRepository{request: request, consumed: map[string]string{hash: "event-purchase-id"}}
	uc := NewRequestUseCaseWithVerifier(repo, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, &scriptedTransferVerifier{outcome: verification.OutcomeConfirmed})
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), hash); err == nil || domainErr.ErrorCode(err) != "transaction_hash_used" {
		t.Fatalf("cross-domain replay error = %v", err)
	}
}

func TestRecoverExpiredPaymentRequestConfirmsOnlyVerifierSuccess(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	hash := strings.Repeat("ab", 32)
	creator := uuid.New()
	payer := uuid.New()
	request := pendingRequest(creator, now)
	request.Status = model.StatusExpired
	request.TransactionHash = &hash
	request.PayerUserID = &payer
	uc := NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: request}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, &scriptedTransferVerifier{outcome: verification.OutcomeConfirmed})
	uc.now = func() time.Time { return now }

	result, err := uc.RecoverExpired(context.Background(), request.PublicID, creator)
	if err != nil {
		t.Fatalf("RecoverExpired: %v", err)
	}
	if result.Data.Status != string(model.StatusPaid) {
		t.Fatalf("status = %q, want paid", result.Data.Status)
	}
	again, err := uc.RecoverExpired(context.Background(), request.PublicID, creator)
	if err != nil {
		t.Fatalf("idempotent recover: %v", err)
	}
	if again.Data.Status != string(model.StatusPaid) {
		t.Fatalf("idempotent status = %q", again.Data.Status)
	}
}

func TestRecoverExpiredPaymentRequestRejectsForeignActor(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	hash := strings.Repeat("cd", 32)
	request := pendingRequest(uuid.New(), now)
	request.Status = model.StatusExpired
	request.TransactionHash = &hash
	uc := NewRequestUseCaseWithVerifier(&fakeRequestRepository{request: request}, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil, &scriptedTransferVerifier{outcome: verification.OutcomeConfirmed})
	if _, err := uc.RecoverExpired(context.Background(), request.PublicID, uuid.New()); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("foreign recover error = %v", err)
	}
}
