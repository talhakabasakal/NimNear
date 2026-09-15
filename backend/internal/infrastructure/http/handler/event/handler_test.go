package event

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	domainEvent "github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type handlerEventRepository struct {
	err error
}

func (f handlerEventRepository) ListPublic(_ context.Context, _ repository.ListFilter) ([]*domainEvent.Event, error) {
	return []*domainEvent.Event{}, f.err
}

func (f handlerEventRepository) ListPublicByOrganizer(_ context.Context, _ uuid.UUID) ([]*domainEvent.Event, error) {
	return nil, nil
}

func (f handlerEventRepository) ListPublicByAttendee(_ context.Context, _ uuid.UUID) ([]*domainEvent.Event, error) {
	return nil, nil
}

func (f handlerEventRepository) GetPublicByID(_ context.Context, _ uuid.UUID) (*domainEvent.Event, error) {
	return nil, f.err
}

func (f handlerEventRepository) Create(_ context.Context, _ *domainEvent.Event) error {
	return nil
}

func TestListReturnsDataEnvelope(t *testing.T) {
	handler := NewHandler(usecase.NewEventUseCase(handlerEventRepository{}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?city=Istanbul", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var response map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := response["data"]; !ok {
		t.Fatal("expected data field")
	}
}

func TestGetReturnsNotFound(t *testing.T) {
	handler := NewHandler(usecase.NewEventUseCase(handlerEventRepository{
		err: domainErr.New(domainErr.ErrNotFound, "event not found", nil),
	}))
	eventID := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/"+eventID, nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", eventID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestCreateRequiresAuthenticatedUser(t *testing.T) {
	handler := NewHandler(usecase.NewEventUseCase(handlerEventRepository{}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", nil)
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
