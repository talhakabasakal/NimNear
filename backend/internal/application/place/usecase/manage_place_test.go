package usecase

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type fakeOperatorRepository struct {
	places map[uuid.UUID]*model.Place
}

func newFakeOperatorRepository() *fakeOperatorRepository {
	return &fakeOperatorRepository{places: map[uuid.UUID]*model.Place{}}
}

func (f *fakeOperatorRepository) Create(_ context.Context, place *model.Place) error {
	if place.ID == uuid.Nil {
		place.ID = uuid.New()
	}
	cloned := *place
	f.places[place.ID] = &cloned
	return nil
}

func (f *fakeOperatorRepository) Update(_ context.Context, place *model.Place) error {
	if _, ok := f.places[place.ID]; !ok {
		return domainErr.New(domainErr.ErrNotFound, "place not found", nil)
	}
	cloned := *place
	f.places[place.ID] = &cloned
	return nil
}

func (f *fakeOperatorRepository) GetByID(_ context.Context, id uuid.UUID) (*model.Place, error) {
	place, ok := f.places[id]
	if !ok {
		return nil, domainErr.New(domainErr.ErrNotFound, "place not found", nil)
	}
	cloned := *place
	return &cloned, nil
}

func (f *fakeOperatorRepository) List(_ context.Context, _ int) ([]*model.Place, error) {
	out := make([]*model.Place, 0, len(f.places))
	for _, place := range f.places {
		cloned := *place
		out = append(out, &cloned)
	}
	return out, nil
}

func (f *fakeOperatorRepository) ListByName(_ context.Context, name string, _ int) ([]*model.Place, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	out := []*model.Place{}
	for _, place := range f.places {
		if strings.ToLower(place.Name) == want {
			cloned := *place
			out = append(out, &cloned)
		}
	}
	return out, nil
}

func TestManagePlaceCreateValidatesAndStores(t *testing.T) {
	uc := NewManagePlaceUseCase(newFakeOperatorRepository())
	result, err := uc.Create(context.Background(), CreatePlaceInput{
		Name: "Example Harbor Cafe", Category: "cafe", Latitude: 41.04, Longitude: 28.99,
		ImageURL: "https://media.example/place.jpg",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.Place.ID == uuid.Nil || !result.Place.IsActive || result.Place.Name != "Example Harbor Cafe" {
		t.Fatalf("unexpected place: %+v", result.Place)
	}
}

func TestManagePlaceCreateRejectsInvalidInputs(t *testing.T) {
	uc := NewManagePlaceUseCase(newFakeOperatorRepository())
	tests := []struct {
		name  string
		input CreatePlaceInput
		want  string
	}{
		{name: "empty name", input: CreatePlaceInput{Latitude: 0, Longitude: 0}, want: "name"},
		{name: "invalid lat", input: CreatePlaceInput{Name: "Cafe", Latitude: 91, Longitude: 0}, want: "latitude"},
		{name: "invalid lon", input: CreatePlaceInput{Name: "Cafe", Latitude: 0, Longitude: 181}, want: "longitude"},
		{name: "nan lat", input: CreatePlaceInput{Name: "Cafe", Latitude: math.NaN(), Longitude: 0}, want: "latitude"},
		{name: "inf lon", input: CreatePlaceInput{Name: "Cafe", Latitude: 0, Longitude: math.Inf(1)}, want: "longitude"},
		{name: "http image", input: CreatePlaceInput{Name: "Cafe", Latitude: 0, Longitude: 0, ImageURL: "http://media.example/x.jpg"}, want: "image_url"},
		{name: "javascript image", input: CreatePlaceInput{Name: "Cafe", Latitude: 0, Longitude: 0, ImageURL: "javascript:alert(1)"}, want: "image_url"},
		{name: "bad category", input: CreatePlaceInput{Name: "Cafe", Latitude: 0, Longitude: 0, Category: "cafe;drop"}, want: "category"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Create(context.Background(), tt.input)
			if err == nil || !errors.Is(err, domainErr.ErrValidation) && !errors.Is(err, domainErr.ErrBadRequest) {
				t.Fatalf("error = %v, want validation/bad request", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestManagePlaceDisableAndListMatchingNames(t *testing.T) {
	repo := newFakeOperatorRepository()
	uc := NewManagePlaceUseCase(repo)
	first, err := uc.Create(context.Background(), CreatePlaceInput{Name: "Shared Name", Latitude: 1, Longitude: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Create(context.Background(), CreatePlaceInput{Name: "shared name", Latitude: 2, Longitude: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Matching) != 1 || second.Matching[0].ID != first.Place.ID {
		t.Fatalf("matching = %+v", second.Matching)
	}
	disabled, err := uc.Disable(context.Background(), first.Place.ID)
	if err != nil || disabled.IsActive {
		t.Fatalf("Disable = %+v err=%v", disabled, err)
	}
}

func TestManagePlaceCreateIsIdempotentForDeterministicID(t *testing.T) {
	uc := NewManagePlaceUseCase(newFakeOperatorRepository())
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	first, err := uc.Create(context.Background(), CreatePlaceInput{ID: id, Name: "Seed Cafe", Latitude: 41, Longitude: 29})
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Create(context.Background(), CreatePlaceInput{ID: id, Name: "Seed Cafe", Latitude: 41, Longitude: 29})
	if err != nil {
		t.Fatal(err)
	}
	if first.Place.ID != id || second.Place.ID != id {
		t.Fatalf("ids = %s %s", first.Place.ID, second.Place.ID)
	}
}
