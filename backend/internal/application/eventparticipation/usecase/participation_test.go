package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventparticipation/model"
	domainRepo "github.com/masterfabric-go/masterfabric/internal/domain/eventparticipation/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type fakeParticipationRepository struct {
	state    *model.State
	err      error
	lastUser uuid.UUID
}

func (f *fakeParticipationRepository) RSVP(_ context.Context, eventID, userID uuid.UUID, _ time.Time) (*model.State, error) {
	f.lastUser = userID
	if f.err != nil {
		return nil, f.err
	}
	if f.state == nil {
		f.state = &model.State{EventID: eventID, Attending: true, AttendeeCount: 1}
	}
	return f.state, nil
}

func (f *fakeParticipationRepository) Cancel(_ context.Context, eventID, userID uuid.UUID) (*model.State, error) {
	f.lastUser = userID
	if f.err != nil {
		return nil, f.err
	}
	if f.state == nil {
		f.state = &model.State{EventID: eventID}
	}
	f.state.Attending = false
	return f.state, nil
}

func (f *fakeParticipationRepository) GetState(_ context.Context, eventID, userID uuid.UUID) (*model.State, error) {
	f.lastUser = userID
	if f.err != nil {
		return nil, f.err
	}
	if f.state == nil {
		f.state = &model.State{EventID: eventID}
	}
	return f.state, nil
}

var _ domainRepo.ParticipationRepository = (*fakeParticipationRepository)(nil)

func TestRSVPSuccess(t *testing.T) {
	eventID := uuid.New()
	userID := uuid.New()
	repo := &fakeParticipationRepository{}
	result, err := NewParticipationUseCase(repo).RSVP(context.Background(), eventID, userID)
	if err != nil {
		t.Fatalf("RSVP returned error: %v", err)
	}
	if !result.Data.Attending || result.Data.EventID != eventID || result.Data.AttendeeCount != 1 {
		t.Fatalf("unexpected RSVP response: %#v", result.Data)
	}
	if repo.lastUser != userID {
		t.Fatalf("repository user = %s, want %s", repo.lastUser, userID)
	}
}

func TestDuplicateRSVPIsIdempotent(t *testing.T) {
	eventID := uuid.New()
	repo := &fakeParticipationRepository{state: &model.State{
		EventID: eventID, Attending: true, AttendeeCount: 1,
	}}
	result, err := NewParticipationUseCase(repo).RSVP(context.Background(), eventID, uuid.New())
	if err != nil {
		t.Fatalf("RSVP returned error: %v", err)
	}
	if !result.Data.Attending || result.Data.AttendeeCount != 1 {
		t.Fatalf("duplicate RSVP changed state: %#v", result.Data)
	}
}

func TestCancelRSVP(t *testing.T) {
	repo := &fakeParticipationRepository{state: &model.State{
		EventID: uuid.New(), Attending: true, AttendeeCount: 1,
	}}
	result, err := NewParticipationUseCase(repo).Cancel(context.Background(), repo.state.EventID, uuid.New())
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if result.Data.Attending || result.Data.AttendeeCount != 1 {
		t.Fatalf("unexpected cancellation response: %#v", result.Data)
	}
}

func TestPaidPastAndSoldOutErrorsArePreserved(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"paid", domainErr.New(domainErr.ErrPaidEvent, "RSVP is available only for free events", nil)},
		{"past", domainErr.New(domainErr.ErrEventPast, "event has already ended", nil)},
		{"sold out", domainErr.New(domainErr.ErrSoldOut, "event is sold out", nil)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParticipationUseCase(&fakeParticipationRepository{err: test.err}).RSVP(context.Background(), uuid.New(), uuid.New())
			if err == nil || err.Error() != test.err.Error() {
				t.Fatalf("error = %v, want %v", err, test.err)
			}
		})
	}
}

func TestRSVPStatusAndCapacity(t *testing.T) {
	capacity := 2
	eventID := uuid.New()
	repo := &fakeParticipationRepository{state: &model.State{
		EventID: eventID, Attending: true, AttendeeCount: 2, Capacity: &capacity,
	}}
	result, err := NewParticipationUseCase(repo).GetState(context.Background(), eventID, uuid.New())
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if !result.Data.Attending || result.Data.AttendeeCount != 2 || !result.Data.IsSoldOut {
		t.Fatalf("unexpected status: %#v", result.Data)
	}
}

func TestRSVPValidationAndNotFound(t *testing.T) {
	uc := NewParticipationUseCase(&fakeParticipationRepository{
		err: domainErr.New(domainErr.ErrNotFound, "event not found", nil),
	})
	if _, err := uc.RSVP(context.Background(), uuid.Nil, uuid.New()); err == nil || !errors.Is(err, domainErr.ErrBadRequest) {
		t.Fatalf("invalid event id error = %v", err)
	}
	if _, err := uc.RSVP(context.Background(), uuid.New(), uuid.Nil); err == nil || !errors.Is(err, domainErr.ErrUnauthorized) {
		t.Fatalf("unauthenticated error = %v", err)
	}
	if _, err := uc.GetState(context.Background(), uuid.New(), uuid.New()); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("not found error = %v", err)
	}
}
