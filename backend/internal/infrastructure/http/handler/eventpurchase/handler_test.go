package eventpurchase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/usecase"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/ratelimit"
)

func TestCreateRateLimitedDoesNotCallUseCase(t *testing.T) {
	userID := uuid.New()
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: userID, AmountLunas: 100000, Status: model.StatusPending}
	uc := usecase.NewPurchaseUseCaseWithVerifier(&handlerRepo{purchase: purchase}, time.Minute, handlerVerifier{}, "merchant", "MainAlbatross").WithIdentities(handlerIdentities{addresses: []string{"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"}})
	handler := NewHandler(uc, ratelimit.NewMemoryLimiter(), Limits{Create: 1, Submit: 1, Window: time.Minute})
	router := chi.NewRouter()
	router.Post("/api/v1/events/{id}/purchases", handler.Create)

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+purchase.EventID.String()+"/purchases", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, userID))
	router.ServeHTTP(first, req)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/events/"+purchase.EventID.String()+"/purchases", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, userID))
	router.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "rate_limited") {
		t.Fatalf("expected rate_limited envelope, got %s", second.Body.String())
	}
}

func TestSubmitRateLimitedDoesNotCallUseCase(t *testing.T) {
	userID := uuid.New()
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: userID, AmountLunas: 100000, Status: model.StatusPending}
	repo := &handlerRepo{purchase: purchase}
	uc := usecase.NewPurchaseUseCaseWithVerifier(repo, time.Minute, handlerVerifier{}, "merchant", "MainAlbatross").WithIdentities(handlerIdentities{addresses: []string{"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"}})
	handler := NewHandler(uc, ratelimit.NewMemoryLimiter(), Limits{Create: 10, Submit: 1, Window: time.Minute})
	router := chi.NewRouter()
	router.Post("/api/v1/purchases/{id}/transaction", handler.SubmitTransaction)

	body := `{"transaction_hash":"` + strings.Repeat("a", 64) + `"}`
	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/purchases/"+purchase.ID.String()+"/transaction", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, userID))
	router.ServeHTTP(first, req)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/purchases/"+purchase.ID.String()+"/transaction", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, userID))
	router.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "rate_limited") {
		t.Fatalf("expected rate_limited envelope, got %s", second.Body.String())
	}
	if repo.submits != 1 {
		t.Fatalf("submit calls = %d, want 1", repo.submits)
	}
}

func TestRecoverExpiredRequiresAuthentication(t *testing.T) {
	handler := NewHandler(nil, nil, Limits{})
	rec := httptest.NewRecorder()
	handler.RecoverExpired(rec, httptest.NewRequest(http.MethodPost, "/api/v1/purchases/"+uuid.New().String()+"/reverify", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type handlerIdentities struct{ addresses []string }

func (s handlerIdentities) VerifiedAddresses(context.Context, uuid.UUID) ([]string, error) {
	return s.addresses, nil
}

type handlerVerifier struct{}

func (handlerVerifier) Verify(context.Context, *model.Purchase) (verification.Outcome, error) {
	return verification.OutcomeNotFinal, nil
}

type handlerRepo struct {
	purchase *model.Purchase
	creates  int
	submits  int
}

func (r *handlerRepo) Create(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Duration) (*model.Purchase, error) {
	r.creates++
	return r.purchase, nil
}
func (r *handlerRepo) GetOwned(context.Context, uuid.UUID, uuid.UUID) (*model.Purchase, error) {
	return r.purchase, nil
}
func (r *handlerRepo) GetActiveForEvent(context.Context, uuid.UUID, uuid.UUID) (*model.Purchase, error) {
	return r.purchase, nil
}
func (r *handlerRepo) SubmitTransaction(context.Context, uuid.UUID, uuid.UUID, string, time.Time, time.Time) (*model.Purchase, error) {
	r.submits++
	return r.purchase, nil
}
func (r *handlerRepo) ClaimVerification(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Duration, time.Duration, bool) (*model.Purchase, error) {
	return r.purchase, nil
}
func (r *handlerRepo) ListReconciliationCandidates(context.Context, time.Time, time.Duration, time.Duration, int) ([]*model.Purchase, error) {
	return nil, nil
}
func (r *handlerRepo) SetVerificationState(context.Context, uuid.UUID, uuid.UUID, model.Status, time.Time) (*model.Purchase, error) {
	return r.purchase, nil
}
func (r *handlerRepo) ExpireUnresolved(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Duration) (*model.Purchase, error) {
	return r.purchase, nil
}
func (r *handlerRepo) ConfirmExpiredRecovery(context.Context, uuid.UUID, uuid.UUID, time.Time) (*model.Purchase, error) {
	return r.purchase, nil
}
