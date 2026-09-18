package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type identityStub struct {
	addresses []string
}

func (s identityStub) VerifiedAddresses(context.Context, uuid.UUID) ([]string, error) {
	return s.addresses, nil
}

type fakePurchaseRepository struct {
	purchase     *model.Purchase
	err          error
	gotEvent     uuid.UUID
	gotUser      uuid.UUID
	gotNow       time.Time
	gotHold      time.Duration
	candidates   []*model.Purchase
	gotListLimit int
	claimCalls   int
	submitCalls  int
}

func (f *fakePurchaseRepository) Create(_ context.Context, eventID, userID uuid.UUID, now time.Time, hold time.Duration) (*model.Purchase, error) {
	f.gotEvent, f.gotUser, f.gotNow, f.gotHold = eventID, userID, now, hold
	return f.purchase, f.err
}

func (f *fakePurchaseRepository) GetOwned(_ context.Context, _, _ uuid.UUID) (*model.Purchase, error) {
	return f.purchase, f.err
}

func (f *fakePurchaseRepository) GetActiveForEvent(_ context.Context, _, _ uuid.UUID) (*model.Purchase, error) {
	return f.purchase, f.err
}

func (f *fakePurchaseRepository) SubmitTransaction(_ context.Context, purchaseID, _ uuid.UUID, hash string, _ time.Time, deadline time.Time) (*model.Purchase, error) {
	f.submitCalls++
	if f.err != nil {
		return nil, f.err
	}
	if f.purchase != nil && f.purchase.ID != purchaseID {
		return nil, domainErr.NewWithCode(domainErr.ErrAlreadyExists, "transaction_hash_used", "transaction hash is already assigned to another purchase", nil)
	}
	if f.purchase != nil {
		f.purchase.TransactionHash = &hash
		f.purchase.Status = model.StatusSubmitted
		f.purchase.ReconciliationDeadlineAt = &deadline
	}
	return f.purchase, nil
}

func (f *fakePurchaseRepository) ClaimVerification(_ context.Context, purchaseID, _ uuid.UUID, _ time.Time, _, _ time.Duration, _ bool) (*model.Purchase, error) {
	f.claimCalls++
	if f.err != nil {
		return nil, f.err
	}
	if len(f.candidates) > 0 {
		for _, candidate := range f.candidates {
			if candidate != nil && candidate.ID == purchaseID {
				return candidate, nil
			}
		}
		return nil, nil
	}
	if f.purchase == nil || (f.purchase.Status != model.StatusSubmitted && f.purchase.Status != model.StatusVerifying) {
		return nil, nil
	}
	return f.purchase, nil
}

func (f *fakePurchaseRepository) ListReconciliationCandidates(_ context.Context, _ time.Time, _, _ time.Duration, limit int) ([]*model.Purchase, error) {
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
	if f.purchase == nil {
		return nil, nil
	}
	return []*model.Purchase{f.purchase}, nil
}

func (f *fakePurchaseRepository) ExpireUnresolved(_ context.Context, purchaseID, _ uuid.UUID, _ time.Time, _ time.Duration) (*model.Purchase, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.candidates) > 0 {
		for _, candidate := range f.candidates {
			if candidate != nil && candidate.ID == purchaseID {
				candidate.Status = model.StatusExpired
				return candidate, nil
			}
		}
	}
	if f.purchase != nil {
		f.purchase.Status = model.StatusExpired
	}
	return f.purchase, nil
}

func (f *fakePurchaseRepository) SetVerificationState(_ context.Context, purchaseID, _ uuid.UUID, status model.Status, _ time.Time) (*model.Purchase, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.candidates) > 0 {
		for _, candidate := range f.candidates {
			if candidate != nil && candidate.ID == purchaseID {
				candidate.Status = status
				return candidate, nil
			}
		}
	}
	if f.purchase != nil {
		f.purchase.Status = status
	}
	return f.purchase, nil
}

func (f *fakePurchaseRepository) ConfirmExpiredRecovery(_ context.Context, purchaseID, _ uuid.UUID, now time.Time) (*model.Purchase, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.purchase == nil || f.purchase.ID != purchaseID {
		return nil, domainErr.New(domainErr.ErrNotFound, "purchase not found", nil)
	}
	if f.purchase.Status == model.StatusConfirmed {
		return f.purchase, nil
	}
	if f.purchase.Status != model.StatusExpired || f.purchase.TransactionHash == nil {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "purchase_not_recoverable", "purchase is not an expired payment with a reserved transaction", nil)
	}
	f.purchase.Status = model.StatusConfirmed
	confirmedAt := now
	f.purchase.ConfirmedAt = &confirmedAt
	return f.purchase, nil
}

func TestPurchaseUseCaseCreateScenarios(t *testing.T) {
	eventID, userID := uuid.New(), uuid.New()
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	purchase := &model.Purchase{
		ID: uuid.New(), EventID: eventID, UserID: userID, AmountLunas: 1250000,
		Status: model.StatusPending, CreatedAt: now, UpdatedAt: now,
	}
	repoErr := errors.New("repository failure")
	identities := identityStub{addresses: []string{"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"}}
	verifier := &scriptedVerifier{outcome: verification.OutcomeNotFinal}

	tests := []struct {
		name       string
		eventID    uuid.UUID
		userID     uuid.UUID
		repo       *fakePurchaseRepository
		configured bool
		wantKind   error
		wantCode   string
		wantNIM    string
		wantLunas  string
		wantStatus string
		wantHold   time.Duration
	}{
		{name: "valid exact paid amount", eventID: eventID, userID: userID, repo: &fakePurchaseRepository{purchase: purchase}, configured: true, wantNIM: "12.5", wantLunas: "1250000", wantStatus: "pending", wantHold: 10 * time.Minute},
		{name: "missing event", eventID: uuid.Nil, userID: userID, repo: &fakePurchaseRepository{purchase: purchase}, wantKind: domainErr.ErrBadRequest},
		{name: "missing user", eventID: eventID, userID: uuid.Nil, repo: &fakePurchaseRepository{purchase: purchase}, wantKind: domainErr.ErrUnauthorized},
		{name: "nil repository", eventID: eventID, userID: userID, wantKind: domainErr.ErrInternal},
		{name: "repository failure", eventID: eventID, userID: userID, repo: &fakePurchaseRepository{err: repoErr}, configured: true, wantKind: repoErr},
		{name: "verifier disabled", eventID: eventID, userID: userID, repo: &fakePurchaseRepository{purchase: purchase}, wantCode: "payment_not_configured"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var repo repository.PurchaseRepository
			if tt.repo != nil {
				repo = tt.repo
			}
			var uc *PurchaseUseCase
			if tt.configured {
				uc = NewPurchaseUseCaseWithVerifier(repo, 0, verifier, "merchant", "MainAlbatross").WithIdentities(identities)
			} else {
				uc = NewPurchaseUseCase(repo, 0)
			}
			uc.now = func() time.Time { return now }
			result, err := uc.Create(context.Background(), tt.eventID, tt.userID)
			if tt.wantCode != "" {
				if err == nil || domainErr.ErrorCode(err) != tt.wantCode {
					t.Fatalf("error = %v, want code %s", err, tt.wantCode)
				}
				if tt.repo != nil && tt.repo.gotEvent != uuid.Nil {
					t.Fatal("create reached the repository when payments were not configured")
				}
				return
			}
			if tt.wantKind != nil {
				if err == nil || (!errors.Is(err, tt.wantKind) && err != tt.wantKind) {
					t.Fatalf("error = %v, want %v", err, tt.wantKind)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}
			if result.Data.AmountNIM != tt.wantNIM || result.Data.AmountLunas != tt.wantLunas || result.Data.Status != tt.wantStatus {
				t.Fatalf("unexpected response: %#v", result.Data)
			}
			if tt.repo.gotEvent != eventID || tt.repo.gotUser != userID || !tt.repo.gotNow.Equal(now) || tt.repo.gotHold != tt.wantHold {
				t.Fatalf("unexpected repository call: %#v", tt.repo)
			}
		})
	}
}

func TestPurchaseUseCaseMapsFractionalLunaWithoutRounding(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakePurchaseRepository{purchase: &model.Purchase{
		ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 1,
		Status: model.StatusPending, CreatedAt: now, UpdatedAt: now,
	}}
	uc := NewPurchaseUseCaseWithVerifier(repo, time.Minute, &scriptedVerifier{}, "merchant", "MainAlbatross").WithIdentities(identityStub{addresses: []string{"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"}})
	result, err := uc.Create(context.Background(), repo.purchase.EventID, repo.purchase.UserID)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.Data.AmountLunas != "1" || result.Data.AmountNIM != "0.00001" {
		t.Fatalf("unexpected minimum exact price: %#v", result.Data)
	}
}

func TestPurchaseUseCaseGetIsOwnerScopedByRepository(t *testing.T) {
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 100000, Status: model.StatusConfirmed}
	result, err := NewPurchaseUseCase(&fakePurchaseRepository{purchase: purchase}, time.Minute).GetOwned(context.Background(), purchase.ID, purchase.UserID)
	if err != nil {
		t.Fatalf("GetOwned returned error: %v", err)
	}
	if result.Data.Status != "confirmed" || result.Data.AmountNIM != "1" {
		t.Fatalf("unexpected owned response: %#v", result.Data)
	}
}

func TestPurchaseUseCaseGetRejectsInvalidIdentity(t *testing.T) {
	uc := NewPurchaseUseCase(&fakePurchaseRepository{}, time.Minute)
	if _, err := uc.GetOwned(context.Background(), uuid.Nil, uuid.New()); !errors.Is(err, domainErr.ErrBadRequest) {
		t.Fatalf("purchase id error = %v", err)
	}
	if _, err := uc.GetOwned(context.Background(), uuid.New(), uuid.Nil); !errors.Is(err, domainErr.ErrUnauthorized) {
		t.Fatalf("user error = %v", err)
	}
}
