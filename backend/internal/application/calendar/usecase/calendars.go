package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/calendar/dto"
	eventdto "github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	eventUseCase "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/calendar/model"
	calendarRepo "github.com/masterfabric-go/masterfabric/internal/domain/calendar/repository"
	eventModel "github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	eventRepo "github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/validator"
)

const (
	DefaultCalendarLimit = 20
	MaxCalendarLimit     = 100
)

// CalendarUseCase handles public calendar reads and legacy-JWT protected mutations.
type CalendarUseCase struct {
	calendarRepo calendarRepo.CalendarRepository
	eventRepo    eventRepo.EventRepository
	now          func() time.Time
}

// NewCalendarUseCase creates a calendar use case.
func NewCalendarUseCase(calendarRepository calendarRepo.CalendarRepository, eventRepository eventRepo.EventRepository) *CalendarUseCase {
	return &CalendarUseCase{
		calendarRepo: calendarRepository,
		eventRepo:    eventRepository,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// ListPublic returns active public calendars in deterministic creation order.
func (uc *CalendarUseCase) ListPublic(ctx context.Context, limit int) (*dto.CalendarsResponse, error) {
	if uc.calendarRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "calendar repository is not configured", nil)
	}
	if limit == 0 {
		limit = DefaultCalendarLimit
	}
	if limit < 1 || limit > MaxCalendarLimit {
		return nil, domainErr.New(domainErr.ErrBadRequest, "limit must be between 1 and 100", nil)
	}
	calendars, err := uc.calendarRepo.ListPublic(ctx, limit)
	if err != nil {
		return nil, err
	}
	return &dto.CalendarsResponse{Data: mapCalendars(calendars)}, nil
}

// GetPublic returns one active public calendar and its published public events.
func (uc *CalendarUseCase) GetPublic(ctx context.Context, id uuid.UUID) (*dto.CalendarResponse, error) {
	if id == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrBadRequest, "calendar id is required", nil)
	}
	if uc.calendarRepo == nil || uc.eventRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "calendar service is not configured", nil)
	}
	calendar, err := uc.calendarRepo.GetPublicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	events, err := uc.eventRepo.ListPublicByCalendar(ctx, id)
	if err != nil {
		return nil, err
	}
	return &dto.CalendarResponse{
		Data:   mapCalendar(calendar),
		Events: mapEvents(events, uc.now().UTC()),
	}, nil
}

// ListMine returns calendars owned by and followed by the authenticated user.
func (uc *CalendarUseCase) ListMine(ctx context.Context, userID uuid.UUID) (*dto.MyCalendarsResponse, error) {
	if userID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil)
	}
	if uc.calendarRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "calendar repository is not configured", nil)
	}
	owned, err := uc.calendarRepo.ListOwned(ctx, userID)
	if err != nil {
		return nil, err
	}
	followed, err := uc.calendarRepo.ListFollowed(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &dto.MyCalendarsResponse{Owned: mapCalendars(owned), Followed: mapCalendars(followed)}, nil
}

// Create creates a calendar owned by the JWT user.
func (uc *CalendarUseCase) Create(ctx context.Context, userID uuid.UUID, req dto.CreateCalendarRequest) (*dto.CalendarResponse, error) {
	if userID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil)
	}
	if uc.calendarRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "calendar repository is not configured", nil)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 255 {
		return nil, domainErr.New(domainErr.ErrValidation, "name must be between 1 and 255 characters", nil)
	}
	if len(req.Description) > 5000 {
		return nil, domainErr.New(domainErr.ErrValidation, "description must be no more than 5000 characters", nil)
	}
	if !validator.ValidMediaURL(req.ImageURL) {
		return nil, domainErr.New(domainErr.ErrValidation, "image_url must be an absolute HTTP(S) URL of no more than 2048 characters", nil)
	}
	visibility := strings.ToLower(strings.TrimSpace(req.Visibility))
	if visibility == "" {
		visibility = string(model.VisibilityPublic)
	}
	if visibility != string(model.VisibilityPublic) && visibility != string(model.VisibilityPrivate) {
		return nil, domainErr.New(domainErr.ErrValidation, "visibility must be public or private", nil)
	}
	now := time.Now().UTC()
	calendar := &model.Calendar{
		ID:          uuid.New(),
		Name:        name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		OwnerID:     userID,
		Visibility:  model.Visibility(visibility),
		Status:      model.StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := uc.calendarRepo.Create(ctx, calendar); err != nil {
		return nil, err
	}
	return &dto.CalendarResponse{Data: mapCalendar(calendar), Events: []eventdto.EventInfo{}}, nil
}

// Follow makes a public calendar available in the user's followed list.
func (uc *CalendarUseCase) Follow(ctx context.Context, userID, calendarID uuid.UUID) error {
	if userID == uuid.Nil {
		return domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil)
	}
	if calendarID == uuid.Nil {
		return domainErr.New(domainErr.ErrBadRequest, "calendar id is required", nil)
	}
	if uc.calendarRepo == nil {
		return domainErr.New(domainErr.ErrInternal, "calendar repository is not configured", nil)
	}
	return uc.calendarRepo.Follow(ctx, calendarID, userID)
}

// Unfollow removes a follow relationship. Repeating the operation is safe.
func (uc *CalendarUseCase) Unfollow(ctx context.Context, userID, calendarID uuid.UUID) error {
	if userID == uuid.Nil {
		return domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil)
	}
	if calendarID == uuid.Nil {
		return domainErr.New(domainErr.ErrBadRequest, "calendar id is required", nil)
	}
	if uc.calendarRepo == nil {
		return domainErr.New(domainErr.ErrInternal, "calendar repository is not configured", nil)
	}
	return uc.calendarRepo.Unfollow(ctx, calendarID, userID)
}

// Update applies owner-only field changes. Followers cannot edit.
func (uc *CalendarUseCase) Update(ctx context.Context, userID, calendarID uuid.UUID, req dto.UpdateCalendarRequest) (*dto.CalendarResponse, error) {
	if userID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil)
	}
	if calendarID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrBadRequest, "calendar id is required", nil)
	}
	if uc.calendarRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "calendar repository is not configured", nil)
	}
	if req.Empty() {
		return nil, domainErr.New(domainErr.ErrValidation, "at least one calendar field is required", nil)
	}
	calendar, err := uc.calendarRepo.GetByID(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	if calendar.OwnerID != userID {
		return nil, domainErr.New(domainErr.ErrForbidden, "calendar is not owned by the authenticated user", nil)
	}
	patch, err := calendarPatch(req)
	if err != nil {
		return nil, err
	}
	updated, err := uc.calendarRepo.UpdateOwned(ctx, calendarID, userID, patch, uc.now().UTC())
	if err != nil {
		return nil, err
	}
	return &dto.CalendarResponse{Data: mapCalendar(updated), Events: []eventdto.EventInfo{}}, nil
}

// Archive unpublishes an owned calendar without deleting event history.
func (uc *CalendarUseCase) Archive(ctx context.Context, userID, calendarID uuid.UUID) (*dto.CalendarResponse, error) {
	if userID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil)
	}
	if calendarID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrBadRequest, "calendar id is required", nil)
	}
	if uc.calendarRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "calendar repository is not configured", nil)
	}
	calendar, err := uc.calendarRepo.GetByID(ctx, calendarID)
	if err != nil {
		return nil, err
	}
	if calendar.OwnerID != userID {
		return nil, domainErr.New(domainErr.ErrForbidden, "calendar is not owned by the authenticated user", nil)
	}
	archived, err := uc.calendarRepo.ArchiveOwned(ctx, calendarID, userID, uc.now().UTC())
	if err != nil {
		return nil, err
	}
	return &dto.CalendarResponse{Data: mapCalendar(archived), Events: []eventdto.EventInfo{}}, nil
}

func calendarPatch(req dto.UpdateCalendarRequest) (model.Patch, error) {
	patch := model.Patch{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" || len(name) > 255 {
			return model.Patch{}, domainErr.New(domainErr.ErrValidation, "name must be between 1 and 255 characters", nil)
		}
		patch.NameSet = true
		patch.Name = name
	}
	if req.Description != nil {
		if len(*req.Description) > 5000 {
			return model.Patch{}, domainErr.New(domainErr.ErrValidation, "description must be no more than 5000 characters", nil)
		}
		patch.DescriptionSet = true
		patch.Description = *req.Description
	}
	if req.ImageURL != nil {
		if !validator.ValidMediaURL(*req.ImageURL) {
			return model.Patch{}, domainErr.New(domainErr.ErrValidation, "image_url must be an absolute HTTP(S) URL of no more than 2048 characters", nil)
		}
		patch.ImageURLSet = true
		patch.ImageURL = *req.ImageURL
	}
	if req.Visibility != nil {
		visibility := strings.ToLower(strings.TrimSpace(*req.Visibility))
		if visibility != string(model.VisibilityPublic) && visibility != string(model.VisibilityPrivate) {
			return model.Patch{}, domainErr.New(domainErr.ErrValidation, "visibility must be public or private", nil)
		}
		patch.VisibilitySet = true
		patch.Visibility = model.Visibility(visibility)
	}
	return patch, nil
}

func mapCalendars(calendars []*model.Calendar) []dto.CalendarInfo {
	result := make([]dto.CalendarInfo, 0, len(calendars))
	for _, calendar := range calendars {
		if calendar != nil {
			result = append(result, mapCalendar(calendar))
		}
	}
	return result
}

func mapCalendar(calendar *model.Calendar) dto.CalendarInfo {
	return dto.CalendarInfo{
		ID:          calendar.ID,
		Name:        calendar.Name,
		Description: optionalText(calendar.Description),
		ImageURL:    optionalMedia(calendar.ImageURL),
		Visibility:  string(calendar.Visibility),
		Status:      string(calendar.Status),
		CreatedAt:   calendar.CreatedAt,
		UpdatedAt:   calendar.UpdatedAt,
	}
}

func mapEvents(events []*eventModel.Event, now time.Time) []eventdto.EventInfo {
	result := make([]eventdto.EventInfo, 0, len(events))
	for _, event := range events {
		if event != nil {
			result = append(result, eventUseCase.MapEvent(event, now))
		}
	}
	return result
}

func optionalText(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func optionalMedia(value string) *string {
	if !validator.ValidMediaURL(value) || value == "" {
		return nil
	}
	return &value
}
