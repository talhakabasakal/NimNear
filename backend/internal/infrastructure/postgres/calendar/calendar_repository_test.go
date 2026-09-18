package calendar

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	calendarmodel "github.com/masterfabric-go/masterfabric/internal/domain/calendar/model"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/testdb"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func testCalendarDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return testdb.Open(t)
}

func TestCalendarRepositoryPublicIsolationAndDeterministicOrdering(t *testing.T) {
	pool := testCalendarDB(t)
	ctx := context.Background()
	repo := NewCalendarRepo(pool)
	ownerID, otherOwnerID := uuid.New(), uuid.New()
	publicID, secondPublicID := uuid.New(), uuid.New()
	privateID, archivedID := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{ownerID, otherOwnerID} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@calendar-test.local"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	insert := func(id, owner uuid.UUID, visibility, status string) {
		_, err := pool.Exec(ctx, `INSERT INTO calendars (id, name, owner_id, visibility, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)`, id, id.String(), owner, visibility, status, now)
		if err != nil {
			t.Fatalf("insert calendar: %v", err)
		}
	}
	insert(publicID, ownerID, "public", "active")
	insert(secondPublicID, ownerID, "public", "active")
	insert(privateID, ownerID, "private", "active")
	insert(archivedID, ownerID, "public", "archived")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM calendars WHERE id = ANY($1)", []uuid.UUID{publicID, secondPublicID, privateID, archivedID})
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{ownerID, otherOwnerID})
	})

	public, err := repo.ListPublic(ctx, 100)
	if err != nil {
		t.Fatalf("ListPublic: %v", err)
	}
	positions := map[uuid.UUID]int{}
	for i, calendar := range public {
		positions[calendar.ID] = i
	}
	if _, ok := positions[privateID]; ok {
		t.Fatal("private calendar leaked into public list")
	}
	if _, ok := positions[archivedID]; ok {
		t.Fatal("archived calendar leaked into public list")
	}
	wantFirst, wantSecond := publicID, secondPublicID
	if bytes.Compare(secondPublicID[:], publicID[:]) < 0 {
		wantFirst, wantSecond = secondPublicID, publicID
	}
	if positions[wantFirst] >= positions[wantSecond] {
		t.Fatalf("same-timestamp public calendars were not ordered by id: %v >= %v", positions[wantFirst], positions[wantSecond])
	}

	if _, err := repo.GetPublicByID(ctx, privateID); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("private detail error = %v, want not found", err)
	}
	owned, err := repo.ListOwned(ctx, ownerID)
	if err != nil {
		t.Fatalf("ListOwned: %v", err)
	}
	ownedIDs := map[uuid.UUID]bool{}
	for _, calendar := range owned {
		ownedIDs[calendar.ID] = true
	}
	if !ownedIDs[privateID] || !ownedIDs[publicID] || !ownedIDs[archivedID] {
		t.Fatalf("owner calendars missing: %v", ownedIDs)
	}
	otherOwned, err := repo.ListOwned(ctx, otherOwnerID)
	if err != nil {
		t.Fatalf("other ListOwned: %v", err)
	}
	for _, calendar := range otherOwned {
		if calendar.ID == publicID {
			t.Fatal("calendar crossed owner boundary")
		}
	}
}

func TestCalendarRepositoryFollowAndUnfollowAreIdempotent(t *testing.T) {
	pool := testCalendarDB(t)
	ctx := context.Background()
	repo := NewCalendarRepo(pool)
	ownerID, followerID, calendarID := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{ownerID, followerID} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@calendar-follow-test.local"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	_, err := pool.Exec(ctx, `INSERT INTO calendars (id, name, owner_id, visibility, status) VALUES ($1, 'Followable', $2, 'public', 'active')`, calendarID, ownerID)
	if err != nil {
		t.Fatalf("insert calendar: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM calendars WHERE id = $1", calendarID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{ownerID, followerID})
	})
	if err := repo.Follow(ctx, calendarID, followerID); err != nil {
		t.Fatalf("first follow: %v", err)
	}
	if err := repo.Follow(ctx, calendarID, followerID); err != nil {
		t.Fatalf("second follow: %v", err)
	}
	followed, err := repo.ListFollowed(ctx, followerID)
	if err != nil || len(followed) != 1 {
		t.Fatalf("followed calendars = %#v, err = %v", followed, err)
	}
	if err := repo.Unfollow(ctx, calendarID, followerID); err != nil {
		t.Fatalf("first unfollow: %v", err)
	}
	if err := repo.Unfollow(ctx, calendarID, followerID); err != nil {
		t.Fatalf("second unfollow: %v", err)
	}
}

func TestCalendarRepositoryOwnerUpdateAndArchiveRejectNonOwner(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	repo := NewCalendarRepo(pool)
	ownerID, otherID, calendarID := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{ownerID, otherID} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@calendar-owner-test.local"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO calendars (id, name, owner_id, visibility, status) VALUES ($1, 'Editable', $2, 'public', 'active')`, calendarID, ownerID); err != nil {
		t.Fatalf("insert calendar: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM calendars WHERE id = $1", calendarID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []uuid.UUID{ownerID, otherID})
	})
	now := time.Now().UTC()
	namePatch := func(name string) calendarmodel.Patch {
		return calendarmodel.Patch{NameSet: true, Name: name}
	}
	updated, err := repo.UpdateOwned(ctx, calendarID, ownerID, namePatch("Owner calendar"), now)
	if err != nil || updated.Name != "Owner calendar" {
		t.Fatalf("owner update = %+v err=%v", updated, err)
	}
	if _, err := repo.UpdateOwned(ctx, calendarID, otherID, namePatch("Hijacked"), now); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("non-owner update error = %v, want not found", err)
	}
	if err := repo.Follow(ctx, calendarID, otherID); err != nil {
		t.Fatalf("follow: %v", err)
	}
	if _, err := repo.UpdateOwned(ctx, calendarID, otherID, namePatch("Follower edit"), now); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("follower update error = %v, want not found", err)
	}
	archived, err := repo.ArchiveOwned(ctx, calendarID, ownerID, now)
	if err != nil || string(archived.Status) != "archived" {
		t.Fatalf("owner archive = %+v err=%v", archived, err)
	}
	if _, err := repo.ArchiveOwned(ctx, calendarID, otherID, now); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("non-owner archive error = %v, want not found", err)
	}
	if _, err := repo.GetPublicByID(ctx, calendarID); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("archived calendar remained public: %v", err)
	}
}
