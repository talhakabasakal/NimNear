package paymentrequest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/nimiqtx/verification"
	"github.com/masterfabric-go/masterfabric/internal/application/paymentrequest/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/ratelimit"
)

const handlerPrimary = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"

type handlerIdentities struct {
	addresses []string
}

func (s handlerIdentities) VerifiedAddresses(context.Context, uuid.UUID) ([]string, error) {
	return s.addresses, nil
}

type handlerRepo struct {
	request *model.Request
}

func (r *handlerRepo) Create(_ context.Context, request *model.Request) (*model.Request, error) {
	cloned := *request
	r.request = &cloned
	return &cloned, nil
}
func (r *handlerRepo) GetByPublicID(_ context.Context, publicID uuid.UUID, now time.Time) (*model.Request, error) {
	if r.request == nil || r.request.PublicID != publicID {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	cloned := *r.request
	if cloned.Status == model.StatusPending && !now.Before(cloned.ExpiresAt) {
		cloned.Status = model.StatusExpired
	}
	return &cloned, nil
}
func (r *handlerRepo) GetOwnedByPublicID(_ context.Context, publicID, creatorID uuid.UUID, now time.Time) (*model.Request, error) {
	request, err := r.GetByPublicID(context.Background(), publicID, now)
	if err != nil {
		return nil, err
	}
	if request.CreatorUserID != creatorID {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	return request, nil
}
func (r *handlerRepo) ListByCreator(_ context.Context, creatorID uuid.UUID, _ time.Time, _ int) ([]*model.Request, error) {
	if r.request == nil || r.request.CreatorUserID != creatorID {
		return []*model.Request{}, nil
	}
	cloned := *r.request
	return []*model.Request{&cloned}, nil
}
func (r *handlerRepo) Cancel(_ context.Context, publicID, creatorID uuid.UUID, now time.Time) (*model.Request, error) {
	request, err := r.GetOwnedByPublicID(context.Background(), publicID, creatorID, now)
	if err != nil {
		return nil, err
	}
	request.Status = model.StatusCancelled
	cancelledAt := now
	request.CancelledAt = &cancelledAt
	r.request.Status = model.StatusCancelled
	return request, nil
}
func (r *handlerRepo) SubmitTransaction(context.Context, uuid.UUID, uuid.UUID, string, time.Time, time.Time) (*model.Request, error) {
	return r.request, nil
}
func (r *handlerRepo) ClaimVerification(context.Context, uuid.UUID, time.Time, time.Duration, time.Duration, bool) (*model.Request, error) {
	return r.request, nil
}
func (r *handlerRepo) ListReconciliationCandidates(context.Context, time.Time, time.Duration, time.Duration, int) ([]*model.Request, error) {
	return nil, nil
}
func (r *handlerRepo) SetVerificationState(context.Context, uuid.UUID, model.Status, time.Time) (*model.Request, error) {
	return r.request, nil
}
func (r *handlerRepo) ExpireUnresolved(context.Context, uuid.UUID, time.Time, time.Duration) (*model.Request, error) {
	return r.request, nil
}
func (r *handlerRepo) ConfirmExpiredRecovery(context.Context, uuid.UUID, time.Time) (*model.Request, error) {
	return r.request, nil
}

type handlerVerifier struct{}

func (handlerVerifier) VerifyTransfer(context.Context, verification.Transfer) (verification.Outcome, error) {
	return verification.OutcomeConfirmed, nil
}

func newHandler(repo *handlerRepo) *Handler {
	if repo == nil {
		repo = &handlerRepo{}
	}
	uc := usecase.NewRequestUseCase(repo, handlerIdentities{addresses: []string{handlerPrimary}}, usecase.DefaultTTL, "test-albatross", nil)
	return NewHandler(uc, ratelimit.NewMemoryLimiter(), Limits{Create: 20, Lookup: 60, Submit: 30, Window: time.Minute})
}

func TestCreateRequiresAuthentication(t *testing.T) {
	handler := newHandler(&handlerRepo{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment-requests", strings.NewReader(`{"amount_nim":"25"}`))
	rec := httptest.NewRecorder()
	handler.Create(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateRejectsClientOwnedPaymentFields(t *testing.T) {
	handler := newHandler(&handlerRepo{})
	userID := uuid.New()
	body := `{"amount_nim":"25","recipient":"NQ07 0000 0000 0000 0000 0000 0000 0000 0000","creator_user_id":"` + userID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment-requests", strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, userID))
	rec := httptest.NewRecorder()
	handler.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateReturnsCreatorRequest(t *testing.T) {
	handler := newHandler(&handlerRepo{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment-requests", strings.NewReader(`{"amount_nim":"25","note":"Dinner"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, uuid.New()))
	rec := httptest.NewRecorder()
	handler.Create(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	data := payload["data"].(map[string]any)
	if data["amount_lunas"] != "2500000" || data["recipient"] != handlerPrimary || data["status"] != "pending" {
		t.Fatalf("data = %#v", data)
	}
	if _, ok := data["creator_user_id"]; ok {
		t.Fatal("creator_user_id leaked")
	}
	if _, ok := data["id"]; ok {
		t.Fatal("internal id leaked")
	}
}

func TestPublicReadOmitsPrivateFields(t *testing.T) {
	now := time.Now().UTC()
	note := "Dinner"
	request := &model.Request{
		ID: uuid.New(), PublicID: uuid.New(), CreatorUserID: uuid.New(),
		RecipientAddress: handlerPrimary, AmountLunas: 2500000, Note: &note,
		Status: model.StatusPending, ExpiresAt: now.Add(usecase.DefaultTTL), CreatedAt: now, UpdatedAt: now,
	}
	handler := newHandler(&handlerRepo{request: request})
	router := chi.NewRouter()
	router.Get("/api/v1/public/payment-requests/{public_id}", handler.GetPublic)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/payment-requests/"+request.PublicID.String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	data := payload["data"].(map[string]any)
	allowed := map[string]bool{"public_id": true, "recipient": true, "amount_lunas": true, "amount_nim": true, "note": true, "status": true, "expires_at": true, "network": true}
	for key := range data {
		if !allowed[key] {
			t.Fatalf("public DTO leaked %q: %#v", key, data)
		}
	}
	for _, hidden := range []string{"creator_user_id", "payer_user_id", "email", "id", "transaction_hash", "cancelled_at", "paid_at"} {
		if _, ok := data[hidden]; ok {
			t.Fatalf("hidden field %q present", hidden)
		}
	}
}

func TestSubmitTransactionRejectsUnknownFields(t *testing.T) {
	now := time.Now().UTC()
	request := &model.Request{
		ID: uuid.New(), PublicID: uuid.New(), CreatorUserID: uuid.New(), RecipientAddress: handlerPrimary,
		AmountLunas: 2500000, Status: model.StatusPending, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	uc := usecase.NewRequestUseCaseWithVerifier(&handlerRepo{request: request}, handlerIdentities{addresses: []string{handlerPrimary}}, usecase.DefaultTTL, "test-albatross", nil, handlerVerifier{})
	handler := NewHandler(uc, nil, Limits{})
	router := chi.NewRouter()
	router.Post("/api/v1/payment-requests/{public_id}/transaction", handler.SubmitTransaction)
	body := `{"transaction_hash":"` + strings.Repeat("a", 64) + `","sender":"` + handlerPrimary + `","amount":"25"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment-requests/"+request.PublicID.String()+"/transaction", bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, uuid.New()))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateRateLimited(t *testing.T) {
	handler := NewHandler(
		usecase.NewRequestUseCase(&handlerRepo{}, handlerIdentities{addresses: []string{handlerPrimary}}, usecase.DefaultTTL, "test-albatross", nil),
		ratelimit.NewMemoryLimiter(),
		Limits{Create: 1, Lookup: 1, Submit: 1, Window: time.Minute},
	)
	userID := uuid.New()
	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment-requests", strings.NewReader(`{"amount_nim":"1"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, userID))
	handler.Create(first, req)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d body=%s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/payment-requests", strings.NewReader(`{"amount_nim":"1"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, userID))
	handler.Create(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d body=%s", second.Code, second.Body.String())
	}
}

func TestUnknownPublicID(t *testing.T) {
	handler := newHandler(&handlerRepo{})
	router := chi.NewRouter()
	router.Get("/api/v1/public/payment-requests/{public_id}", handler.GetPublic)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/payment-requests/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecoverExpiredRequiresAuthentication(t *testing.T) {
	handler := NewHandler(nil, nil, Limits{})
	rec := httptest.NewRecorder()
	handler.RecoverExpired(rec, httptest.NewRequest(http.MethodPost, "/api/v1/payment-requests/"+uuid.New().String()+"/reverify", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

var _ repository.RequestRepository = (*handlerRepo)(nil)
