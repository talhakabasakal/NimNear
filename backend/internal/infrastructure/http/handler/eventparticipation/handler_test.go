package eventparticipation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	applicationUC "github.com/masterfabric-go/masterfabric/internal/application/eventparticipation/usecase"
	domainModel "github.com/masterfabric-go/masterfabric/internal/domain/eventparticipation/model"
	domainRepo "github.com/masterfabric-go/masterfabric/internal/domain/eventparticipation/repository"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
)

type handlerRepository struct{}

func (handlerRepository) RSVP(_ context.Context, eventID, _ uuid.UUID, _ time.Time) (*domainModel.State, error) {
	return &domainModel.State{EventID: eventID, Attending: true, AttendeeCount: 1}, nil
}

func (handlerRepository) Cancel(_ context.Context, eventID, _ uuid.UUID) (*domainModel.State, error) {
	return &domainModel.State{EventID: eventID, Attending: false}, nil
}

func (handlerRepository) GetState(_ context.Context, eventID, _ uuid.UUID) (*domainModel.State, error) {
	return &domainModel.State{EventID: eventID, Attending: false}, nil
}

var _ domainRepo.ParticipationRepository = handlerRepository{}

func TestRSVPRequiresAuthentication(t *testing.T) {
	handler := NewHandler(applicationUC.NewParticipationUseCase(handlerRepository{}))
	eventID := uuid.NewString()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID+"/rsvp", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", eventID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	rec := httptest.NewRecorder()

	handler.RSVP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRSVPReturnsAuthenticatedState(t *testing.T) {
	handler := NewHandler(applicationUC.NewParticipationUseCase(handlerRepository{}))
	eventID := uuid.NewString()
	userID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID+"/rsvp", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", eventID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeContext)
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.RSVP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
