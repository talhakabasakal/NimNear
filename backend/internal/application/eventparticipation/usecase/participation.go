package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/eventparticipation/dto"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventparticipation/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventparticipation/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// ParticipationUseCase handles authenticated event RSVP operations.
type ParticipationUseCase struct {
	repo repository.ParticipationRepository
	now  func() time.Time
}

// NewParticipationUseCase creates a participation use case using the system clock.
func NewParticipationUseCase(repo repository.ParticipationRepository) *ParticipationUseCase {
	return &ParticipationUseCase{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// RSVP adds the authenticated user to a free, public, published, upcoming event.
func (uc *ParticipationUseCase) RSVP(ctx context.Context, eventID, userID uuid.UUID) (*dto.ParticipationResponse, error) {
	if err := validateIDs(eventID, userID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "participation repository is not configured", nil)
	}
	state, err := uc.repo.RSVP(ctx, eventID, userID, uc.now().UTC())
	if err != nil {
		return nil, err
	}
	return mapResponse(state), nil
}

// Cancel removes only the authenticated user's participation.
func (uc *ParticipationUseCase) Cancel(ctx context.Context, eventID, userID uuid.UUID) (*dto.ParticipationResponse, error) {
	if err := validateIDs(eventID, userID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "participation repository is not configured", nil)
	}
	state, err := uc.repo.Cancel(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	return mapResponse(state), nil
}

// GetState returns the authenticated user's participation state without exposing
// it through the public event response.
func (uc *ParticipationUseCase) GetState(ctx context.Context, eventID, userID uuid.UUID) (*dto.ParticipationResponse, error) {
	if err := validateIDs(eventID, userID); err != nil {
		return nil, err
	}
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "participation repository is not configured", nil)
	}
	state, err := uc.repo.GetState(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	return mapResponse(state), nil
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

func mapResponse(state *model.State) *dto.ParticipationResponse {
	if state == nil {
		return &dto.ParticipationResponse{}
	}
	return &dto.ParticipationResponse{Data: dto.ParticipationInfo{
		EventID:       state.EventID,
		Attending:     state.Attending,
		AttendeeCount: state.AttendeeCount,
		Capacity:      state.Capacity,
		IsSoldOut:     state.IsSoldOut(),
	}}
}
