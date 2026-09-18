package place

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	placeUC "github.com/masterfabric-go/masterfabric/internal/application/place/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/model"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/testdb"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func TestPlaceOperatorCreateNearbyDisableAndDetail(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	repo := NewPlaceRepo(pool)
	uc := placeUC.NewManagePlaceUseCase(repo)
	originLat, originLon := 41.0400, 28.9900

	near, err := uc.Create(ctx, placeUC.CreatePlaceInput{
		Name: "Nearby Example Cafe", Category: "cafe", Latitude: originLat, Longitude: originLon,
		ImageURL: "https://example.com/media/nearby.jpg",
	})
	if err != nil {
		t.Fatalf("create nearby: %v", err)
	}
	far, err := uc.Create(ctx, placeUC.CreatePlaceInput{
		Name: "Far Example Hall", Category: "venue", Latitude: 42.0400, Longitude: 28.9900,
	})
	if err != nil {
		t.Fatalf("create far: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM events WHERE place_id = ANY($1)", []uuid.UUID{near.Place.ID, far.Place.ID})
		_, _ = pool.Exec(ctx, "DELETE FROM places WHERE id = ANY($1)", []uuid.UUID{near.Place.ID, far.Place.ID})
	})

	detail, err := repo.GetActiveByID(ctx, near.Place.ID)
	if err != nil || detail.Name != "Nearby Example Cafe" {
		t.Fatalf("GetActiveByID = %+v err=%v", detail, err)
	}

	nearby, err := repo.ListNearby(ctx, originLat, originLon, 5000, 100)
	if err != nil {
		t.Fatalf("ListNearby: %v", err)
	}
	if !containsPlace(nearbyIDs(nearby), near.Place.ID) {
		t.Fatal("nearby place missing from radius search")
	}
	if containsPlace(nearbyIDs(nearby), far.Place.ID) {
		t.Fatal("far place leaked into 5km radius")
	}

	limited, err := repo.ListNearby(ctx, originLat, originLon, 50000, 1)
	if err != nil {
		t.Fatalf("limited ListNearby: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("limit = %d, want 1", len(limited))
	}

	if _, err := uc.Disable(ctx, near.Place.ID); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if _, err := repo.GetActiveByID(ctx, near.Place.ID); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("disabled detail error = %v, want not found", err)
	}
	afterDisable, err := repo.ListNearby(ctx, originLat, originLon, 5000, 100)
	if err != nil {
		t.Fatalf("ListNearby after disable: %v", err)
	}
	if containsPlace(nearbyIDs(afterDisable), near.Place.ID) {
		t.Fatal("disabled place leaked into nearby")
	}
	operator, err := repo.GetByID(ctx, near.Place.ID)
	if err != nil || operator.IsActive {
		t.Fatalf("operator GetByID = %+v err=%v", operator, err)
	}
}

func TestPlaceEventsAreDiscoverableByPlaceID(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	repo := NewPlaceRepo(pool)
	uc := placeUC.NewManagePlaceUseCase(repo)
	created, err := uc.Create(ctx, placeUC.CreatePlaceInput{
		Name: "Event Host Cafe", Category: "cafe", Latitude: 41.041, Longitude: 28.991,
	})
	if err != nil {
		t.Fatalf("create place: %v", err)
	}
	eventID, organizerID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", organizerID, organizerID.String()+"@place-event-test.local"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	starts := time.Now().UTC().Add(time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO events (id, title, description, starts_at, ends_at, status, price_lunas, currency, city, place_id, organizer_id, is_public)
		VALUES ($1, 'Place-hosted example event', '', $2, $3, 'published', 0, 'NIM', 'Example City', $4, $5, TRUE)`,
		eventID, starts, starts.Add(time.Hour), created.Place.ID, organizerID); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM events WHERE id = $1", eventID)
		_, _ = pool.Exec(ctx, "DELETE FROM places WHERE id = $1", created.Place.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", organizerID)
	})

	var found uuid.UUID
	err = pool.QueryRow(ctx, `SELECT id FROM events WHERE place_id = $1 AND is_public = TRUE AND status = 'published'`, created.Place.ID).Scan(&found)
	if err != nil || found != eventID {
		t.Fatalf("place-associated event = %s err=%v", found, err)
	}
}

func nearbyIDs(places []*model.NearbyPlace) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(places))
	for _, place := range places {
		ids = append(ids, place.ID)
	}
	return ids
}

func containsPlace(ids []uuid.UUID, id uuid.UUID) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}
