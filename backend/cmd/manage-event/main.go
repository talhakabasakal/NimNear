package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	eventdto "github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	eventUC "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	pgCalendar "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/calendar"
	pgEvent "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/event"
	pgPlace "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/place"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "manage-event: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: manage-event <list|get|cancel> [flags]")
	}
	uc, closer, err := newUseCase()
	if err != nil {
		return err
	}
	defer closer()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "list":
		return listEvents(ctx, uc, args[1:])
	case "get":
		return getEvent(ctx, uc, args[1:])
	case "cancel", "takedown":
		return cancelEvent(ctx, uc, args[1:])
	default:
		return fmt.Errorf("usage: manage-event <list|get|cancel> [flags]")
	}
}

func newUseCase() (*eventUC.EventUseCase, func(), error) {
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
	uc := eventUC.NewEventUseCaseWithAssociations(
		pgEvent.NewEventRepo(db),
		pgPlace.NewPlaceRepo(db),
		pgCalendar.NewCalendarRepo(db),
	)
	return uc, db.Close, nil
}

func listEvents(ctx context.Context, uc *eventUC.EventUseCase, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	city := fs.String("city", "", "optional city filter")
	limit := fs.Int("limit", 50, "max events to list")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := uc.List(ctx, eventdto.ListEventsQuery{City: strings.TrimSpace(*city), Limit: *limit})
	if err != nil {
		return err
	}
	for _, event := range result.Data {
		printEvent(event)
	}
	fmt.Printf("%d event(s)\n", len(result.Data))
	return nil
}

func getEvent(ctx context.Context, uc *eventUC.EventUseCase, args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	idValue := fs.String("id", "", "event UUID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := uuid.Parse(strings.TrimSpace(*idValue))
	if err != nil {
		return fmt.Errorf("id must be a UUID")
	}
	result, err := uc.Get(ctx, id)
	if err != nil {
		return err
	}
	printEvent(result.Data)
	return nil
}

func cancelEvent(ctx context.Context, uc *eventUC.EventUseCase, args []string) error {
	fs := flag.NewFlagSet("cancel", flag.ContinueOnError)
	idValue := fs.String("id", "", "event UUID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	id, err := uuid.Parse(strings.TrimSpace(*idValue))
	if err != nil {
		return fmt.Errorf("id must be a UUID")
	}
	result, err := uc.CancelByOperator(ctx, id)
	if err != nil {
		return err
	}
	fmt.Print("cancelled ")
	printEvent(result.Data)
	return nil
}

func printEvent(event eventdto.EventInfo) {
	organizer := "none"
	if event.OrganizerID != nil {
		organizer = event.OrganizerID.String()
	}
	fmt.Printf("%s  %s  status=%s  starts=%s  organizer=%s  attendees=%d\n",
		event.ID, event.Title, event.Status, event.StartsAt.UTC().Format(time.RFC3339), organizer, event.AttendeeCount)
}
