package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	eventUC "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	pgCalendar "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/calendar"
	pgEvent "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/event"
	pgIam "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/iam"
	pgPlace "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/place"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "seed-production-events: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("seed-production-events", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	organizerValue := fs.String("organizer-id", "", "required production users.id UUID of the organizer (not a Nimiq address)")
	production := fs.Bool("production", false, "required confirmation to insert into a production database")
	dryRun := fs.Bool("dry-run", false, "print the plan without creating events")
	if err := fs.Parse(args); err != nil {
		return err
	}

	organizerID, err := uuid.Parse(strings.TrimSpace(*organizerValue))
	if err != nil || organizerID == uuid.Nil {
		return fmt.Errorf("organizer-id is required: pass the production users.id UUID of the organizer. A Nimiq wallet address is not accepted")
	}

	environment, databaseConfig := config.LoadDatabaseConfig()
	if err := databaseConfig.ValidateForEnvironment(environment); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := database.NewPostgresPool(ctx, databaseConfig)
	if err != nil {
		return fmt.Errorf("postgres unavailable")
	}
	defer db.Close()

	eventRepo := pgEvent.NewEventRepo(db)
	s := &seeder{
		isProduction: environment == config.EnvironmentProduction,
		now:          time.Now().UTC(),
		users:        pgIam.NewUserRepo(db),
		events: eventUC.NewEventUseCaseWithAssociations(
			eventRepo,
			pgPlace.NewPlaceRepo(db),
			pgCalendar.NewCalendarRepo(db),
		),
		existing:    eventRepo,
		verifyImage: verifyHTTPImage(nil),
		stdout:      os.Stdout,
	}
	return s.apply(ctx, options{
		organizerID: organizerID,
		production:  *production,
		dryRun:      *dryRun,
	})
}
