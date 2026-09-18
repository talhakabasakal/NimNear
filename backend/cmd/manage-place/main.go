package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	placeUC "github.com/masterfabric-go/masterfabric/internal/application/place/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/model"
	pgPlace "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/place"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "manage-place: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: manage-place <create|update|disable|list> [flags]")
	}
	uc, closer, err := newUseCase()
	if err != nil {
		return err
	}
	defer closer()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "create":
		return createPlace(ctx, uc, args[1:])
	case "update":
		return updatePlace(ctx, uc, args[1:])
	case "disable":
		return disablePlace(ctx, uc, args[1:])
	case "list":
		return listPlaces(ctx, uc, args[1:])
	default:
		return fmt.Errorf("usage: manage-place <create|update|disable|list> [flags]")
	}
}

func newUseCase() (*placeUC.ManagePlaceUseCase, func(), error) {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("postgres unavailable")
	}
	return placeUC.NewManagePlaceUseCase(pgPlace.NewPlaceRepo(db)), db.Close, nil
}

func createPlace(ctx context.Context, uc *placeUC.ManagePlaceUseCase, args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	input, err := parseCreateFlags(fs, args)
	if err != nil {
		return err
	}
	result, err := uc.Create(ctx, input)
	if err != nil {
		return err
	}
	if len(result.Matching) > 0 {
		fmt.Printf("warning: %d existing place(s) share this name; UUID is the authoritative identity\n", len(result.Matching))
		for _, place := range result.Matching {
			printPlace(place)
		}
	}
	fmt.Print("created ")
	printPlace(result.Place)
	return nil
}

func updatePlace(ctx context.Context, uc *placeUC.ManagePlaceUseCase, args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	idValue := fs.String("id", "", "place UUID")
	name := fs.String("name", "", "place name")
	description := fs.String("description", "", "description")
	address := fs.String("address", "", "address")
	category := fs.String("category", "", "category")
	imageURL := fs.String("image-url", "", "HTTPS image URL")
	lat := fs.Float64("lat", 0, "latitude")
	lon := fs.Float64("lon", 0, "longitude")
	active := fs.String("active", "", "true or false")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := uuid.Parse(strings.TrimSpace(*idValue))
	if err != nil {
		return fmt.Errorf("id must be a UUID")
	}
	input := placeUC.UpdatePlaceInput{ID: id}
	var activeErr error
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "name":
			input.Name = name
		case "description":
			input.Description = description
		case "address":
			input.Address = address
		case "category":
			input.Category = category
		case "image-url":
			input.ImageURL = imageURL
		case "lat":
			input.Latitude = lat
		case "lon":
			input.Longitude = lon
		case "active":
			value, parseErr := strconv.ParseBool(*active)
			if parseErr != nil {
				activeErr = fmt.Errorf("active must be true or false")
				return
			}
			input.IsActive = &value
		}
	})
	if activeErr != nil {
		return activeErr
	}
	place, err := uc.Update(ctx, input)
	if err != nil {
		return err
	}
	fmt.Print("updated ")
	printPlace(place)
	return nil
}

func disablePlace(ctx context.Context, uc *placeUC.ManagePlaceUseCase, args []string) error {
	fs := flag.NewFlagSet("disable", flag.ContinueOnError)
	idValue := fs.String("id", "", "place UUID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := uuid.Parse(strings.TrimSpace(*idValue))
	if err != nil {
		return fmt.Errorf("id must be a UUID")
	}
	place, err := uc.Disable(ctx, id)
	if err != nil {
		return err
	}
	fmt.Print("disabled ")
	printPlace(place)
	return nil
}

func listPlaces(ctx context.Context, uc *placeUC.ManagePlaceUseCase, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	name := fs.String("name", "", "optional exact name filter")
	if err := fs.Parse(args); err != nil {
		return err
	}
	places, err := uc.List(ctx, 200)
	if err != nil {
		return err
	}
	filter := strings.ToLower(strings.TrimSpace(*name))
	count := 0
	for _, place := range places {
		if filter != "" && strings.ToLower(place.Name) != filter {
			continue
		}
		printPlace(place)
		count++
	}
	fmt.Printf("%d place(s)\n", count)
	return nil
}

func parseCreateFlags(fs *flag.FlagSet, args []string) (placeUC.CreatePlaceInput, error) {
	name := fs.String("name", "", "place name")
	description := fs.String("description", "", "description")
	address := fs.String("address", "", "address")
	category := fs.String("category", "", "category")
	imageURL := fs.String("image-url", "", "HTTPS image URL")
	lat := fs.Float64("lat", 0, "latitude")
	lon := fs.Float64("lon", 0, "longitude")
	active := fs.Bool("active", true, "active flag")
	idValue := fs.String("id", "", "optional deterministic UUID")
	if err := fs.Parse(args); err != nil {
		return placeUC.CreatePlaceInput{}, err
	}
	input := placeUC.CreatePlaceInput{
		Name: *name, Description: *description, Address: *address, Category: *category,
		ImageURL: *imageURL, Latitude: *lat, Longitude: *lon, IsActive: active,
	}
	if strings.TrimSpace(*idValue) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*idValue))
		if err != nil {
			return placeUC.CreatePlaceInput{}, fmt.Errorf("id must be a UUID")
		}
		input.ID = id
	}
	return input, nil
}

func printPlace(place *model.Place) {
	fmt.Printf("%s  %s  %.6f,%.6f  category=%s  active=%t\n", place.ID, place.Name, place.Latitude, place.Longitude, place.Category, place.IsActive)
}
