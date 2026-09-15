package usecase

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const (
	DefaultEventLimit = 20
	MaxEventLimit     = 100
)

var pricePattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

const LunasPerNIM int64 = 100000

// EventUseCase handles public event discovery and authenticated creation.
type EventUseCase struct {
	eventRepo repository.EventRepository
	now       func() time.Time
}

// NewEventUseCase creates an event use case using the system clock.
func NewEventUseCase(eventRepo repository.EventRepository) *EventUseCase {
	return &EventUseCase{eventRepo: eventRepo, now: func() time.Time { return time.Now().UTC() }}
}

// List returns public events. With no time window it defaults to upcoming events.
func (uc *EventUseCase) List(ctx context.Context, query dto.ListEventsQuery) (*dto.EventsResponse, error) {
	if uc.eventRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "event repository is not configured", nil)
	}
	if query.Limit == 0 {
		query.Limit = DefaultEventLimit
	}
	if query.Limit < 1 || query.Limit > MaxEventLimit {
		return nil, domainErr.New(domainErr.ErrBadRequest, "limit must be between 1 and 100", nil)
	}
	query.City = strings.TrimSpace(query.City)
	if len(query.City) > 120 {
		return nil, domainErr.New(domainErr.ErrBadRequest, "city must be no more than 120 characters", nil)
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return nil, domainErr.New(domainErr.ErrBadRequest, "from must be before or equal to to", nil)
	}
	if query.From == nil && query.To == nil {
		now := uc.now().UTC()
		query.From = &now
	}

	events, err := uc.eventRepo.ListPublic(ctx, repository.ListFilter{
		City:  query.City,
		From:  query.From,
		To:    query.To,
		Limit: query.Limit,
	})
	if err != nil {
		return nil, err
	}

	now := uc.now().UTC()
	data := make([]dto.EventInfo, 0, len(events))
	for _, event := range events {
		if event != nil {
			data = append(data, MapEvent(event, now))
		}
	}
	return &dto.EventsResponse{Data: data}, nil
}

// Get returns one public event by ID.
func (uc *EventUseCase) Get(ctx context.Context, id uuid.UUID) (*dto.EventResponse, error) {
	if id == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrBadRequest, "event id is required", nil)
	}
	if uc.eventRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "event repository is not configured", nil)
	}
	event, err := uc.eventRepo.GetPublicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, domainErr.New(domainErr.ErrNotFound, "event not found", nil)
	}
	return &dto.EventResponse{Data: MapEvent(event, uc.now().UTC())}, nil
}

// Create creates a published public event owned by the authenticated user.
func (uc *EventUseCase) Create(ctx context.Context, organizerID uuid.UUID, req dto.CreateEventRequest) (*dto.EventResponse, error) {
	if organizerID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil)
	}
	if uc.eventRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "event repository is not configured", nil)
	}
	event, err := buildEvent(organizerID, req)
	if err != nil {
		return nil, err
	}
	if err := uc.eventRepo.Create(ctx, event); err != nil {
		return nil, err
	}
	return &dto.EventResponse{Data: MapEvent(event, uc.now().UTC())}, nil
}

func buildEvent(organizerID uuid.UUID, req dto.CreateEventRequest) (*model.Event, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" || len(title) > 255 {
		return nil, domainErr.New(domainErr.ErrValidation, "title must be between 1 and 255 characters", nil)
	}
	if len(req.Description) > 5000 {
		return nil, domainErr.New(domainErr.ErrValidation, "description must be no more than 5000 characters", nil)
	}
	if req.StartsAt.IsZero() || req.EndsAt.IsZero() {
		return nil, domainErr.New(domainErr.ErrValidation, "starts_at and ends_at are required", nil)
	}
	if !req.EndsAt.After(req.StartsAt) {
		return nil, domainErr.New(domainErr.ErrValidation, "ends_at must be after starts_at", nil)
	}
	priceLunas, err := parsePriceLunas(req.PriceNIM)
	if err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "NIM"
	}
	if len(currency) > 16 {
		return nil, domainErr.New(domainErr.ErrValidation, "currency must be no more than 16 characters", nil)
	}
	if req.Capacity != nil && *req.Capacity < 1 {
		return nil, domainErr.New(domainErr.ErrValidation, "capacity must be greater than 0", nil)
	}
	if err := validateCoordinates(req.Latitude, req.Longitude); err != nil {
		return nil, err
	}
	city := strings.TrimSpace(req.City)
	if city == "" || len(city) > 120 {
		return nil, domainErr.New(domainErr.ErrValidation, "city must be between 1 and 120 characters", nil)
	}
	if req.Address != nil && len(*req.Address) > 500 {
		return nil, domainErr.New(domainErr.ErrValidation, "address must be no more than 500 characters", nil)
	}
	if len(req.ImageURL) > 2048 {
		return nil, domainErr.New(domainErr.ErrValidation, "image_url must be no more than 2048 characters", nil)
	}

	now := time.Now().UTC()
	return &model.Event{
		ID:            uuid.New(),
		Title:         title,
		Description:   req.Description,
		StartsAt:      req.StartsAt.UTC(),
		EndsAt:        req.EndsAt.UTC(),
		Status:        model.EventStatusPublished,
		PriceLunas:    priceLunas,
		Currency:      currency,
		Capacity:      req.Capacity,
		AttendeeCount: 0,
		ImageURL:      req.ImageURL,
		PlaceID:       req.PlaceID,
		Latitude:      req.Latitude,
		Longitude:     req.Longitude,
		Address:       req.Address,
		City:          city,
		OrganizerID:   &organizerID,
		IsPublic:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func MapEvent(event *model.Event, now time.Time) dto.EventInfo {
	return dto.EventInfo{
		ID:            event.ID,
		Title:         event.Title,
		Description:   event.Description,
		StartsAt:      event.StartsAt,
		EndsAt:        event.EndsAt,
		Status:        string(event.Status),
		PriceNIM:      formatPriceNIM(event.PriceLunas),
		Currency:      event.Currency,
		Capacity:      event.Capacity,
		AttendeeCount: event.AttendeeCount,
		ImageURL:      event.ImageURL,
		PlaceID:       event.PlaceID,
		Latitude:      event.Latitude,
		Longitude:     event.Longitude,
		Address:       event.Address,
		City:          event.City,
		OrganizerID:   event.OrganizerID,
		IsFree:        event.IsFree(),
		IsSoldOut:     event.IsSoldOut(),
		IsPast:        event.IsPast(now),
		CreatedAt:     event.CreatedAt,
		UpdatedAt:     event.UpdatedAt,
	}
}

func parsePriceLunas(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	if !pricePattern.MatchString(raw) {
		return 0, domainErr.New(domainErr.ErrValidation, "price_nim must be a non-negative decimal with no more than 5 decimal places; no rounding is applied", nil)
	}
	parts := strings.SplitN(raw, ".", 2)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 5 {
		return 0, domainErr.New(domainErr.ErrValidation, "price_nim must be a non-negative decimal with no more than 5 decimal places; no rounding is applied", nil)
	}
	integerPart := new(big.Int)
	if _, ok := integerPart.SetString(parts[0], 10); !ok {
		return 0, domainErr.New(domainErr.ErrValidation, "price_nim must be a non-negative decimal with no more than 5 decimal places; no rounding is applied", nil)
	}
	fraction = fraction + strings.Repeat("0", 5-len(fraction))
	fractionPart := new(big.Int)
	if fraction != "" {
		if _, ok := fractionPart.SetString(fraction, 10); !ok {
			return 0, domainErr.New(domainErr.ErrValidation, "price_nim must be a non-negative decimal with no more than 5 decimal places; no rounding is applied", nil)
		}
	}
	total := new(big.Int).Mul(integerPart, big.NewInt(LunasPerNIM))
	total.Add(total, fractionPart)
	maxInt64 := new(big.Int).SetInt64(int64(^uint64(0) >> 1))
	if total.Sign() < 0 || total.Cmp(maxInt64) > 0 {
		return 0, domainErr.New(domainErr.ErrValidation, "price_nim is too large", nil)
	}
	return total.Int64(), nil
}

func formatPriceNIM(lunas int64) string {
	if lunas == 0 {
		return "0"
	}
	whole := lunas / LunasPerNIM
	fraction := lunas % LunasPerNIM
	if fraction == 0 {
		return fmt.Sprintf("%d", whole)
	}
	return strings.TrimRight(fmt.Sprintf("%d.%05d", whole, fraction), "0")
}

func validateCoordinates(latitude, longitude *float64) error {
	if (latitude == nil) != (longitude == nil) {
		return domainErr.New(domainErr.ErrValidation, "latitude and longitude must be provided together", nil)
	}
	if latitude == nil {
		return nil
	}
	if math.IsNaN(*latitude) || math.IsInf(*latitude, 0) || *latitude < -90 || *latitude > 90 {
		return domainErr.New(domainErr.ErrValidation, "latitude must be between -90 and 90", nil)
	}
	if math.IsNaN(*longitude) || math.IsInf(*longitude, 0) || *longitude < -180 || *longitude > 180 {
		return domainErr.New(domainErr.ErrValidation, "longitude must be between -180 and 180", nil)
	}
	return nil
}
