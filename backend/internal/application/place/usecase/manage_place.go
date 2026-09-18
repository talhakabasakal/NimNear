package usecase

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/validator"
)

const (
	maxPlaceNameLength        = 255
	maxPlaceDescriptionLength = 5000
	maxPlaceAddressLength     = 500
	maxPlaceCategoryLength    = 100
	maxOperatorList           = 200
)

var placeCategoryPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// ManagePlaceUseCase is the operator inventory path for places.
type ManagePlaceUseCase struct {
	repo repository.OperatorRepository
}

// NewManagePlaceUseCase creates an operator place inventory use case.
func NewManagePlaceUseCase(repo repository.OperatorRepository) *ManagePlaceUseCase {
	return &ManagePlaceUseCase{repo: repo}
}

// CreatePlaceInput is operator-supplied place data.
type CreatePlaceInput struct {
	ID          uuid.UUID
	Name        string
	Description string
	Latitude    float64
	Longitude   float64
	Address     string
	Category    string
	ImageURL    string
	IsActive    *bool
}

// UpdatePlaceInput patches an existing place. Nil pointers leave fields unchanged.
type UpdatePlaceInput struct {
	ID          uuid.UUID
	Name        *string
	Description *string
	Latitude    *float64
	Longitude   *float64
	Address     *string
	Category    *string
	ImageURL    *string
	IsActive    *bool
}

// CreatePlaceResult is the created place plus same-name matches for operators.
type CreatePlaceResult struct {
	Place    *model.Place
	Matching []*model.Place
}

// Create validates and inserts a place. Duplicate names are allowed; UUID is identity.
func (uc *ManagePlaceUseCase) Create(ctx context.Context, input CreatePlaceInput) (*CreatePlaceResult, error) {
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "place repository is not configured", nil)
	}
	place, err := validateNewPlace(input)
	if err != nil {
		return nil, err
	}
	matching, err := uc.repo.ListByName(ctx, place.Name, maxOperatorList)
	if err != nil {
		return nil, err
	}
	if input.ID != uuid.Nil {
		existing, getErr := uc.repo.GetByID(ctx, input.ID)
		if getErr == nil && existing != nil {
			return &CreatePlaceResult{Place: existing, Matching: matching}, nil
		}
		if getErr != nil && !isNotFound(getErr) {
			return nil, getErr
		}
	}
	if err := uc.repo.Create(ctx, place); err != nil {
		return nil, err
	}
	return &CreatePlaceResult{Place: place, Matching: matching}, nil
}

// Update applies a validated patch to an existing place, including inactive ones.
func (uc *ManagePlaceUseCase) Update(ctx context.Context, input UpdatePlaceInput) (*model.Place, error) {
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "place repository is not configured", nil)
	}
	if input.ID == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrValidation, "place id is required", nil)
	}
	place, err := uc.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	applyPlacePatch(place, input)
	if err := validatePlaceRecord(place); err != nil {
		return nil, err
	}
	if err := uc.repo.Update(ctx, place); err != nil {
		return nil, err
	}
	return place, nil
}

// Disable marks a place inactive. Public nearby and detail reads then exclude it.
func (uc *ManagePlaceUseCase) Disable(ctx context.Context, id uuid.UUID) (*model.Place, error) {
	inactive := false
	return uc.Update(ctx, UpdatePlaceInput{ID: id, IsActive: &inactive})
}

// List returns operator inventory, including inactive places.
func (uc *ManagePlaceUseCase) List(ctx context.Context, limit int) ([]*model.Place, error) {
	if uc.repo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "place repository is not configured", nil)
	}
	if limit <= 0 || limit > maxOperatorList {
		limit = maxOperatorList
	}
	return uc.repo.List(ctx, limit)
}

func validateNewPlace(input CreatePlaceInput) (*model.Place, error) {
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}
	place := &model.Place{
		ID:          input.ID,
		Name:        input.Name,
		Description: input.Description,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		Address:     input.Address,
		Category:    input.Category,
		ImageURL:    input.ImageURL,
		IsActive:    active,
	}
	if err := validatePlaceRecord(place); err != nil {
		return nil, err
	}
	return place, nil
}

func applyPlacePatch(place *model.Place, input UpdatePlaceInput) {
	if input.Name != nil {
		place.Name = *input.Name
	}
	if input.Description != nil {
		place.Description = *input.Description
	}
	if input.Latitude != nil {
		place.Latitude = *input.Latitude
	}
	if input.Longitude != nil {
		place.Longitude = *input.Longitude
	}
	if input.Address != nil {
		place.Address = *input.Address
	}
	if input.Category != nil {
		place.Category = *input.Category
	}
	if input.ImageURL != nil {
		place.ImageURL = *input.ImageURL
	}
	if input.IsActive != nil {
		place.IsActive = *input.IsActive
	}
}

func validatePlaceRecord(place *model.Place) error {
	place.Name = strings.TrimSpace(place.Name)
	if place.Name == "" || utf8.RuneCountInString(place.Name) > maxPlaceNameLength {
		return domainErr.New(domainErr.ErrValidation, "name must be between 1 and 255 characters", nil)
	}
	place.Description = strings.TrimSpace(place.Description)
	if utf8.RuneCountInString(place.Description) > maxPlaceDescriptionLength {
		return domainErr.New(domainErr.ErrValidation, "description must be no more than 5000 characters", nil)
	}
	place.Address = strings.TrimSpace(place.Address)
	if utf8.RuneCountInString(place.Address) > maxPlaceAddressLength {
		return domainErr.New(domainErr.ErrValidation, "address must be no more than 500 characters", nil)
	}
	place.Category = strings.TrimSpace(place.Category)
	if utf8.RuneCountInString(place.Category) > maxPlaceCategoryLength {
		return domainErr.New(domainErr.ErrValidation, "category must be no more than 100 characters", nil)
	}
	if place.Category != "" && !placeCategoryPattern.MatchString(place.Category) {
		return domainErr.New(domainErr.ErrValidation, "category must start with a letter and contain only letters, digits, hyphen, or underscore", nil)
	}
	place.ImageURL = strings.TrimSpace(place.ImageURL)
	if !validator.ValidHTTPSMediaURL(place.ImageURL) {
		return domainErr.New(domainErr.ErrValidation, "image_url must be empty or an absolute HTTPS URL of no more than 2048 characters", nil)
	}
	return validateCoordinates(place.Latitude, place.Longitude)
}

func isNotFound(err error) bool {
	return errors.Is(err, domainErr.ErrNotFound)
}
