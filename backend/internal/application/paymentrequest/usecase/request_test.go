package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	iamUC "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/application/paymentrequest/dto"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const (
	primaryIdentity = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"
	secondIdentity  = "NQ07 0000 0000 0000 0000 0000 0000 0000 0000"
)

type identityStub struct {
	addresses []string
	err       error
}

func (s identityStub) VerifiedAddresses(context.Context, uuid.UUID) ([]string, error) {
	return s.addresses, s.err
}

type fakeRequestRepository struct {
	mu             sync.Mutex
	request        *model.Request
	requests       []*model.Request
	err            error
	consumed       map[string]string
	submitCalls    int
	claimCalls     int
	candidates     []*model.Request
	gotListLimit   int
	created        *model.Request
	cancelConflict error
}

func (f *fakeRequestRepository) Create(_ context.Context, request *model.Request) (*model.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	cloned := *request
	f.created = &cloned
	f.request = &cloned
	return &cloned, nil
}

func (f *fakeRequestRepository) GetByPublicID(_ context.Context, publicID uuid.UUID, now time.Time) (*model.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getByPublicIDLocked(publicID, now)
}

func (f *fakeRequestRepository) getByPublicIDLocked(publicID uuid.UUID, now time.Time) (*model.Request, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.request == nil || f.request.PublicID != publicID {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	cloned := *f.request
	if cloned.Status == model.StatusPending && !now.Before(cloned.ExpiresAt) {
		cloned.Status = model.StatusExpired
		f.request.Status = model.StatusExpired
	}
	return &cloned, nil
}

func (f *fakeRequestRepository) GetOwnedByPublicID(_ context.Context, publicID, creatorID uuid.UUID, now time.Time) (*model.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	request, err := f.getByPublicIDLocked(publicID, now)
	if err != nil {
		return nil, err
	}
	if request.CreatorUserID != creatorID {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	return request, nil
}

func (f *fakeRequestRepository) ListByCreator(_ context.Context, creatorID uuid.UUID, now time.Time, limit int) ([]*model.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotListLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	source := f.requests
	if len(source) == 0 && f.request != nil {
		source = []*model.Request{f.request}
	}
	out := make([]*model.Request, 0, len(source))
	for _, request := range source {
		if request == nil || request.CreatorUserID != creatorID {
			continue
		}
		cloned := *request
		if cloned.Status == model.StatusPending && !now.Before(cloned.ExpiresAt) {
			cloned.Status = model.StatusExpired
			request.Status = model.StatusExpired
		}
		out = append(out, &cloned)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeRequestRepository) Cancel(_ context.Context, publicID, creatorID uuid.UUID, now time.Time) (*model.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if f.cancelConflict != nil {
		return nil, f.cancelConflict
	}
	if f.request == nil || f.request.PublicID != publicID || f.request.CreatorUserID != creatorID {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	if f.request.Status == model.StatusPending && !now.Before(f.request.ExpiresAt) {
		f.request.Status = model.StatusExpired
		f.request.UpdatedAt = now
		cloned := *f.request
		return &cloned, nil
	}
	switch f.request.Status {
	case model.StatusCancelled, model.StatusExpired:
		cloned := *f.request
		return &cloned, nil
	case model.StatusPending:
		f.request.Status = model.StatusCancelled
		cancelledAt := now
		f.request.CancelledAt = &cancelledAt
		f.request.UpdatedAt = now
		cloned := *f.request
		return &cloned, nil
	default:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_cancellable", "payment request can no longer be cancelled", nil)
	}
}

func (f *fakeRequestRepository) SubmitTransaction(_ context.Context, publicID, payerID uuid.UUID, hash string, now, deadline time.Time) (*model.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submitCalls++
	if f.err != nil {
		return nil, f.err
	}
	if f.consumed != nil {
		if owner, ok := f.consumed[hash]; ok && (f.request == nil || owner != f.request.ID.String()) {
			return nil, domainErr.NewWithCode(domainErr.ErrAlreadyExists, "transaction_hash_used", "transaction hash is already consumed", nil)
		}
	}
	if f.request == nil || f.request.PublicID != publicID {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	if f.request.Status == model.StatusPending && !now.Before(f.request.ExpiresAt) {
		f.request.Status = model.StatusExpired
	}
	if f.request.TransactionHash != nil && *f.request.TransactionHash == hash &&
		f.request.PayerUserID != nil && *f.request.PayerUserID == payerID &&
		(f.request.Status == model.StatusSubmitted || f.request.Status == model.StatusVerifying || f.request.Status == model.StatusPaid) {
		cloned := *f.request
		return &cloned, nil
	}
	switch f.request.Status {
	case model.StatusPaid:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_already_paid", "payment request is already paid", nil)
	case model.StatusCancelled:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_cancelled", "payment request is cancelled", nil)
	case model.StatusExpired:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_expired", "payment request has expired", nil)
	case model.StatusPending:
	default:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_submittable", "payment request is not waiting for a transaction", nil)
	}
	if f.submitCalls > 1 && f.request.Status == model.StatusSubmitted {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_submittable", "payment request is not waiting for a transaction", nil)
	}
	f.request.Status = model.StatusSubmitted
	f.request.TransactionHash = &hash
	payer := payerID
	f.request.PayerUserID = &payer
	f.request.ReconciliationDeadlineAt = &deadline
	if f.consumed == nil {
		f.consumed = map[string]string{}
	}
	f.consumed[hash] = f.request.ID.String()
	cloned := *f.request
	return &cloned, nil
}

func (f *fakeRequestRepository) ClaimVerification(_ context.Context, id uuid.UUID, _ time.Time, _, _ time.Duration, _ bool) (*model.Request, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimCalls++
	if f.err != nil {
		return nil, f.err
	}
	if len(f.candidates) > 0 {
		for _, candidate := range f.candidates {
			if candidate != nil && candidate.ID == id {
				return candidate, nil
			}
		}
		return nil, nil
	}
	if f.request == nil || f.request.ID != id || (f.request.Status != model.StatusSubmitted && f.request.Status != model.StatusVerifying) {
		return nil, nil
	}
	return f.request, nil
}

func (f *fakeRequestRepository) ListReconciliationCandidates(_ context.Context, _ time.Time, _, _ time.Duration, limit int) ([]*model.Request, error) {
	f.gotListLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	if len(f.candidates) > 0 {
		if len(f.candidates) > limit {
			return f.candidates[:limit], nil
		}
		return f.candidates, nil
	}
	if f.request == nil {
		return nil, nil
	}
	return []*model.Request{f.request}, nil
}

func (f *fakeRequestRepository) ExpireUnresolved(_ context.Context, id uuid.UUID, _ time.Time, _ time.Duration) (*model.Request, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.candidates) > 0 {
		for _, candidate := range f.candidates {
			if candidate != nil && candidate.ID == id {
				candidate.Status = model.StatusExpired
				return candidate, nil
			}
		}
	}
	if f.request != nil {
		f.request.Status = model.StatusExpired
		cloned := *f.request
		return &cloned, nil
	}
	return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
}

func (f *fakeRequestRepository) SetVerificationState(_ context.Context, id uuid.UUID, status model.Status, now time.Time) (*model.Request, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.candidates) > 0 {
		for _, candidate := range f.candidates {
			if candidate != nil && candidate.ID == id {
				candidate.Status = status
				if status == model.StatusPaid {
					paidAt := now
					candidate.PaidAt = &paidAt
				}
				return candidate, nil
			}
		}
	}
	if f.request != nil {
		f.request.Status = status
		if status == model.StatusPaid {
			paidAt := now
			f.request.PaidAt = &paidAt
		}
		cloned := *f.request
		return &cloned, nil
	}
	return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
}

func (f *fakeRequestRepository) ConfirmExpiredRecovery(_ context.Context, id uuid.UUID, now time.Time) (*model.Request, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.request == nil || f.request.ID != id {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	if f.request.Status == model.StatusPaid {
		cloned := *f.request
		return &cloned, nil
	}
	if f.request.Status != model.StatusExpired || f.request.TransactionHash == nil {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_recoverable", "payment request is not an expired payment with a reserved transaction", nil)
	}
	f.request.Status = model.StatusPaid
	paidAt := now
	f.request.PaidAt = &paidAt
	cloned := *f.request
	return &cloned, nil
}

var _ repository.RequestRepository = (*fakeRequestRepository)(nil)

func newUseCase(t *testing.T, repo *fakeRequestRepository, identities identityStub) *RequestUseCase {
	t.Helper()
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	uc := NewRequestUseCase(repo, identities, DefaultTTL, "test-albatross", nil)
	uc.now = func() time.Time { return now }
	return uc
}

func TestCreateResolvesVerifiedRecipientAndStoresLuna(t *testing.T) {
	repo := &fakeRequestRepository{}
	uc := newUseCase(t, repo, identityStub{addresses: []string{primaryIdentity, secondIdentity}})
	creatorID := uuid.New()

	result, err := uc.Create(context.Background(), creatorID, dto.CreateRequest{AmountNIM: "25", Note: "  Dinner  "})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Data.AmountLunas != "2500000" || result.Data.AmountNIM != "25" {
		t.Fatalf("amount = %#v", result.Data)
	}
	if result.Data.Recipient != primaryIdentity {
		t.Fatalf("recipient = %q, want most recent verified identity", result.Data.Recipient)
	}
	if result.Data.Note == nil || *result.Data.Note != "Dinner" {
		t.Fatalf("note = %#v", result.Data.Note)
	}
	if result.Data.Status != "pending" || result.Data.PublicID == uuid.Nil || result.Data.Network != "test-albatross" {
		t.Fatalf("unexpected request: %#v", result.Data)
	}
	if result.Data.ExpiresAt != uc.now().UTC().Add(DefaultTTL) {
		t.Fatalf("expires_at = %s", result.Data.ExpiresAt)
	}
	if repo.created == nil || repo.created.CreatorUserID != creatorID {
		t.Fatal("creator was not persisted from the authenticated user")
	}
}

func TestCreateUsesSelectedVerifiedIdentity(t *testing.T) {
	repo := &fakeRequestRepository{}
	uc := newUseCase(t, repo, identityStub{addresses: []string{primaryIdentity, secondIdentity}})
	result, err := uc.Create(context.Background(), uuid.New(), dto.CreateRequest{AmountNIM: "1", Address: secondIdentity})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Data.Recipient != secondIdentity {
		t.Fatalf("recipient = %q", result.Data.Recipient)
	}
}

func TestCreateRejectsForeignIdentityAndMissingIdentity(t *testing.T) {
	foreign := iamUC.AddressFromPublicKey(bytesRepeat(7))
	uc := newUseCase(t, &fakeRequestRepository{}, identityStub{addresses: []string{primaryIdentity}})
	if _, err := uc.Create(context.Background(), uuid.New(), dto.CreateRequest{AmountNIM: "1", Address: foreign}); err == nil || !errors.Is(err, domainErr.ErrForbidden) {
		t.Fatalf("foreign identity error = %v", err)
	}
	uc = newUseCase(t, &fakeRequestRepository{}, identityStub{})
	if _, err := uc.Create(context.Background(), uuid.New(), dto.CreateRequest{AmountNIM: "1"}); err == nil || domainErr.ErrorCode(err) != "no_verified_nimiq_identity" {
		t.Fatalf("missing identity error = %v", err)
	}
}

func TestCreateRejectsUnauthenticatedCreator(t *testing.T) {
	uc := newUseCase(t, &fakeRequestRepository{}, identityStub{addresses: []string{primaryIdentity}})
	if _, err := uc.Create(context.Background(), uuid.Nil, dto.CreateRequest{AmountNIM: "1"}); !errors.Is(err, domainErr.ErrUnauthorized) {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateAmountValidation(t *testing.T) {
	uc := newUseCase(t, &fakeRequestRepository{}, identityStub{addresses: []string{primaryIdentity}})
	tests := []struct {
		name   string
		amount string
		code   string
	}{
		{name: "zero", amount: "0", code: "invalid_amount"},
		{name: "negative", amount: "-1", code: "invalid_amount"},
		{name: "malformed", amount: "12.5.0", code: "invalid_amount"},
		{name: "too many decimals", amount: "1.000001", code: "invalid_amount"},
		{name: "overflow", amount: "92233720368548", code: "amount_too_large"},
		{name: "empty", amount: " ", code: "invalid_amount"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Create(context.Background(), uuid.New(), dto.CreateRequest{AmountNIM: tt.amount})
			if err == nil || domainErr.ErrorCode(err) != tt.code {
				t.Fatalf("error = %v, want code %q", err, tt.code)
			}
		})
	}
}

func TestCreateExactFiveDecimalPlaces(t *testing.T) {
	uc := newUseCase(t, &fakeRequestRepository{}, identityStub{addresses: []string{primaryIdentity}})
	result, err := uc.Create(context.Background(), uuid.New(), dto.CreateRequest{AmountNIM: "0.00001"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Data.AmountLunas != "1" || result.Data.AmountNIM != "0.00001" {
		t.Fatalf("amount = %#v", result.Data)
	}
}

func TestCreateNoteLimitAndControlCharacters(t *testing.T) {
	uc := newUseCase(t, &fakeRequestRepository{}, identityStub{addresses: []string{primaryIdentity}})
	long := strings.Repeat("a", MaxNoteLength+1)
	if utf8.RuneCountInString(long) <= MaxNoteLength {
		t.Fatal("test setup")
	}
	if _, err := uc.Create(context.Background(), uuid.New(), dto.CreateRequest{AmountNIM: "1", Note: long}); err == nil || domainErr.ErrorCode(err) != "note_too_long" {
		t.Fatalf("long note error = %v", err)
	}
	if _, err := uc.Create(context.Background(), uuid.New(), dto.CreateRequest{AmountNIM: "1", Note: "hi\nthere"}); err == nil || domainErr.ErrorCode(err) != "invalid_note" {
		t.Fatalf("control note error = %v", err)
	}
}

func TestCreatorListAndDetailAreOwnerScoped(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	creatorID := uuid.New()
	request := pendingRequest(creatorID, now)
	repo := &fakeRequestRepository{request: request, requests: []*model.Request{request}}
	uc := NewRequestUseCase(repo, identityStub{addresses: []string{primaryIdentity}}, DefaultTTL, "test-albatross", nil)
	uc.now = func() time.Time { return now }

	list, err := uc.ListOwned(context.Background(), creatorID, 0)
	if err != nil {
		t.Fatalf("ListOwned: %v", err)
	}
	if len(list.Data) != 1 || list.Data[0].PublicID != request.PublicID {
		t.Fatalf("list = %#v", list.Data)
	}
	if repo.gotListLimit != DefaultListLimit {
		t.Fatalf("limit = %d", repo.gotListLimit)
	}
	got, err := uc.GetOwned(context.Background(), request.PublicID, creatorID)
	if err != nil {
		t.Fatalf("GetOwned: %v", err)
	}
	if got.Data.PublicID != request.PublicID {
		t.Fatalf("detail = %#v", got.Data)
	}
	if _, err := uc.GetOwned(context.Background(), request.PublicID, uuid.New()); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("foreign detail error = %v", err)
	}
}

func TestPublicReadHidesCreatorAndExposesOnlyPayerFields(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	creatorID := uuid.New()
	note := "Dinner"
	request := pendingRequest(creatorID, now)
	request.Note = &note
	uc := NewRequestUseCase(&fakeRequestRepository{request: request}, identityStub{}, DefaultTTL, "test-albatross", nil)
	uc.now = func() time.Time { return now }

	got, err := uc.GetPublic(context.Background(), request.PublicID)
	if err != nil {
		t.Fatalf("GetPublic: %v", err)
	}
	if got.Data.PublicID != request.PublicID || got.Data.Recipient != primaryIdentity || got.Data.AmountLunas != "2500000" || got.Data.Note == nil || got.Data.Network != "test-albatross" {
		t.Fatalf("public = %#v", got.Data)
	}
	if _, err := uc.GetPublic(context.Background(), uuid.New()); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("unknown public id error = %v", err)
	}
}

func TestExpiredPendingIsVisibleAsExpiredBeforeWorker(t *testing.T) {
	created := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	now := created.Add(DefaultTTL)
	request := pendingRequest(uuid.New(), created)
	uc := NewRequestUseCase(&fakeRequestRepository{request: request}, identityStub{}, DefaultTTL, "test-albatross", nil)
	uc.now = func() time.Time { return now }
	got, err := uc.GetPublic(context.Background(), request.PublicID)
	if err != nil {
		t.Fatalf("GetPublic: %v", err)
	}
	if got.Data.Status != string(model.StatusExpired) {
		t.Fatalf("status = %q, want expired", got.Data.Status)
	}
}

func TestCancelPendingAndIdempotentCancelledExpired(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	creatorID := uuid.New()
	request := pendingRequest(creatorID, now)
	repo := &fakeRequestRepository{request: request}
	uc := NewRequestUseCase(repo, identityStub{}, DefaultTTL, "test-albatross", nil)
	uc.now = func() time.Time { return now }

	got, err := uc.Cancel(context.Background(), request.PublicID, creatorID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Data.Status != "cancelled" || got.Data.CancelledAt == nil {
		t.Fatalf("cancelled = %#v", got.Data)
	}
	again, err := uc.Cancel(context.Background(), request.PublicID, creatorID)
	if err != nil {
		t.Fatalf("idempotent cancel: %v", err)
	}
	if again.Data.Status != "cancelled" {
		t.Fatalf("status = %q", again.Data.Status)
	}
	if _, err := uc.Cancel(context.Background(), request.PublicID, uuid.New()); !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("non-creator cancel error = %v", err)
	}

	paid := pendingRequest(creatorID, now)
	paid.Status = model.StatusPaid
	uc = NewRequestUseCase(&fakeRequestRepository{request: paid}, identityStub{}, DefaultTTL, "test-albatross", nil)
	uc.now = func() time.Time { return now }
	if _, err := uc.Cancel(context.Background(), paid.PublicID, creatorID); err == nil || domainErr.ErrorCode(err) != "payment_request_not_cancellable" {
		t.Fatalf("paid cancel error = %v", err)
	}

	expired := pendingRequest(creatorID, now.Add(-DefaultTTL))
	uc = NewRequestUseCase(&fakeRequestRepository{request: expired}, identityStub{}, DefaultTTL, "test-albatross", nil)
	uc.now = func() time.Time { return now }
	result, err := uc.Cancel(context.Background(), expired.PublicID, creatorID)
	if err != nil {
		t.Fatalf("expired cancel: %v", err)
	}
	if result.Data.Status != "expired" {
		t.Fatalf("expired cancel status = %q", result.Data.Status)
	}
}

func pendingRequest(creatorID uuid.UUID, now time.Time) *model.Request {
	return &model.Request{
		ID:               uuid.New(),
		PublicID:         uuid.New(),
		CreatorUserID:    creatorID,
		RecipientAddress: primaryIdentity,
		AmountLunas:      2500000,
		Status:           model.StatusPending,
		ExpiresAt:        now.Add(DefaultTTL),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func bytesRepeat(value byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = value
	}
	return out
}
