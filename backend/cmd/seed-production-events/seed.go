package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	eventmodel "github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	iammodel "github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	"github.com/masterfabric-go/masterfabric/internal/shared/validator"
)

type options struct {
	organizerID uuid.UUID
	production  bool
	dryRun      bool
}

type eventCreator interface {
	Create(ctx context.Context, organizerID uuid.UUID, req dto.CreateEventRequest) (*dto.EventResponse, error)
}

type organizerEventLister interface {
	ListPublicByOrganizer(ctx context.Context, organizerID uuid.UUID) ([]*eventmodel.Event, error)
}

type userLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*iammodel.User, error)
}

type imageVerifier func(ctx context.Context, imageURL string) error

type plannedEvent struct {
	catalog   catalogEvent
	request   dto.CreateEventRequest
	action    string
	existing  *eventmodel.Event
	smokeTest bool
}

type seeder struct {
	isProduction bool
	now          time.Time
	users        userLookup
	events       eventCreator
	existing     organizerEventLister
	verifyImage  imageVerifier
	stdout       io.Writer
}

func (s *seeder) apply(ctx context.Context, opts options) error {
	if opts.organizerID == uuid.Nil {
		return fmt.Errorf("organizer-id is required: pass the production users.id UUID of the organizer. A Nimiq wallet address is not accepted")
	}
	if opts.production && !s.isProduction {
		return fmt.Errorf("--production requires APP_ENV=production")
	}
	if !opts.dryRun && (!s.isProduction || !opts.production) {
		return fmt.Errorf("refusing to write: production inserts require APP_ENV=production and --production; use --dry-run to preview")
	}

	user, err := s.users.GetByID(ctx, opts.organizerID)
	if err != nil {
		return fmt.Errorf("organizer lookup failed: %w", err)
	}
	if user == nil || !user.IsActive() {
		return fmt.Errorf("organizer %s is missing, inactive, or deleted; provide an active users.id UUID", opts.organizerID)
	}

	existing, err := s.existing.ListPublicByOrganizer(ctx, opts.organizerID)
	if err != nil {
		return fmt.Errorf("list existing organizer events: %w", err)
	}
	present := indexByTitle(existing)

	loc, err := time.LoadLocation(catalogTZ)
	if err != nil {
		return fmt.Errorf("load timezone %s: %w", catalogTZ, err)
	}

	catalog := catalogEvents()
	plan := make([]plannedEvent, 0, len(catalog))
	for _, item := range catalog {
		if err := validateCatalogItem(item); err != nil {
			return err
		}
		startsAt, endsAt := item.schedule(s.now, loc)
		req := item.request(startsAt, endsAt)
		planned := plannedEvent{catalog: item, request: req, action: "create", smokeTest: item.SmokeTest}
		if found := present[normalizeTitle(item.Title)]; found != nil {
			planned.action = "skip"
			planned.existing = found
		}
		plan = append(plan, planned)
	}

	for _, item := range plan {
		if s.verifyImage == nil {
			break
		}
		if err := s.verifyImage(ctx, item.catalog.ImageURL); err != nil {
			return fmt.Errorf("image for %q did not resolve: %w", item.catalog.Title, err)
		}
	}

	if err := printPlan(s.stdout, opts, s.isProduction, user.ID, plan); err != nil {
		return err
	}
	if opts.dryRun {
		fmt.Fprintln(s.stdout, "dry-run: no events were created")
		return nil
	}

	created, skipped := 0, 0
	for _, item := range plan {
		if item.action != "create" {
			skipped++
			fmt.Fprintf(s.stdout, "skip %s (already present as %s)\n", item.catalog.Title, item.existing.ID)
			continue
		}
		result, err := s.events.Create(ctx, opts.organizerID, item.request)
		if err != nil {
			return fmt.Errorf("create %q: %w", item.catalog.Title, err)
		}
		created++
		fmt.Fprintf(s.stdout, "created %s %s\n", result.Data.ID, result.Data.Title)
	}
	fmt.Fprintf(s.stdout, "done created=%d skipped=%d\n", created, skipped)
	return nil
}

func validateCatalogItem(item catalogEvent) error {
	if strings.TrimSpace(item.Title) == "" {
		return fmt.Errorf("catalog title is required")
	}
	if item.Duration <= 0 {
		return fmt.Errorf("%q: duration must be positive", item.Title)
	}
	if item.Capacity < 1 {
		return fmt.Errorf("%q: capacity must be greater than 0", item.Title)
	}
	if !validator.ValidMediaURL(item.ImageURL) || item.ImageURL == "" {
		return fmt.Errorf("%q: image_url is not a valid HTTP(S) URL", item.Title)
	}
	return nil
}

func indexByTitle(events []*eventmodel.Event) map[string]*eventmodel.Event {
	index := make(map[string]*eventmodel.Event, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		key := normalizeTitle(event.Title)
		if key == "" {
			continue
		}
		if _, exists := index[key]; !exists {
			index[key] = event
		}
	}
	return index
}

func normalizeTitle(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

func printPlan(w io.Writer, opts options, isProduction bool, organizerID uuid.UUID, plan []plannedEvent) error {
	mode := "WRITE"
	if opts.dryRun {
		mode = "DRY-RUN"
	}
	env := "non-production"
	if isProduction {
		env = "production"
	}
	fmt.Fprintf(w, "seed-production-events mode=%s env=%s organizer=%s\n", mode, env, organizerID)
	fmt.Fprintln(w, "existing published events are never deleted, cancelled, or updated")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACTION\tTITLE\tTHEME\tLOCATION\tSTARTS_AT\tPRICE\tIMAGE")
	for _, item := range plan {
		price := item.catalog.PriceNIM
		if item.catalog.PriceNIM == "" || item.catalog.PriceNIM == "0" {
			price = "FREE"
		} else {
			price = item.catalog.PriceNIM + " NIM"
		}
		if item.smokeTest {
			price += " [1-Luna smoke-test]"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			item.action,
			item.catalog.Title,
			item.catalog.Theme,
			item.catalog.Address,
			item.request.StartsAt.UTC().Format(time.RFC3339),
			price,
			item.catalog.ImageSource,
		)
	}
	return tw.Flush()
}

func verifyHTTPImage(client *http.Client) imageVerifier {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return func(ctx context.Context, imageURL string) error {
		if err := probeImage(ctx, client, http.MethodHead, imageURL); err == nil {
			return nil
		}
		return probeImage(ctx, client, http.MethodGet, imageURL)
	}
}

func probeImage(ctx context.Context, client *http.Client, method, imageURL string) error {
	req, err := http.NewRequestWithContext(ctx, method, imageURL, nil)
	if err != nil {
		return err
	}
	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}
	req.Header.Set("User-Agent", "nimnear-seed-production-events/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.HasPrefix(contentType, "image/") && !strings.Contains(contentType, "octet-stream") {
		return fmt.Errorf("unexpected content type %s", contentType)
	}
	return nil
}
