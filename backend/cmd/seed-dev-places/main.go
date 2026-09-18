package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	placeUC "github.com/masterfabric-go/masterfabric/internal/application/place/usecase"
	pgPlace "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/place"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/database"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// Development-only fictional inventory. Never runs during server startup or
// production. IDs are deterministic so the command is idempotent and non-destructive.
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed-dev-places: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.IsProduction() {
		return fmt.Errorf("refusing to seed fictional places in production")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("postgres unavailable")
	}
	defer db.Close()

	repo := pgPlace.NewPlaceRepo(db)
	uc := placeUC.NewManagePlaceUseCase(repo)
	for _, seed := range devPlaces() {
		existing, err := repo.GetByID(ctx, seed.ID)
		if err == nil && existing != nil {
			fmt.Printf("present %s %s\n", existing.ID, existing.Name)
			continue
		}
		if err != nil && !errors.Is(err, domainErr.ErrNotFound) {
			return err
		}
		result, err := uc.Create(ctx, seed)
		if err != nil {
			return err
		}
		fmt.Printf("created %s %s\n", result.Place.ID, result.Place.Name)
	}
	return nil
}

func devPlaces() []placeUC.CreatePlaceInput {
	active := true
	return []placeUC.CreatePlaceInput{
		{
			ID:   uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			Name: "Example Harbor Cafe", Description: "Fictional development cafe near the example harbor.",
			Latitude: 41.0400, Longitude: 28.9900, Address: "1 Example Harbor Walk",
			Category: "cafe", ImageURL: "https://example.com/media/harbor-cafe.jpg", IsActive: &active,
		},
		{
			ID:   uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			Name: "Example Garden Hall", Description: "Fictional indoor hall for development events.",
			Latitude: 41.0420, Longitude: 28.9920, Address: "12 Example Garden Street",
			Category: "venue", ImageURL: "https://example.com/media/garden-hall.jpg", IsActive: &active,
		},
		{
			ID:   uuid.MustParse("33333333-3333-4333-8333-333333333333"),
			Name: "Example North Market", Description: "Fictional open market used only in development.",
			Latitude: 41.0380, Longitude: 28.9880, Address: "7 Example North Square",
			Category: "market", IsActive: &active,
		},
	}
}
