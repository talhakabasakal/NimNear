package iam

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	infraAuth "github.com/masterfabric-go/masterfabric/internal/infrastructure/auth"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/profile"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/testdb"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

func TestAccountDeletionAnonymizesAndPreservesEvidence(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	now := time.Now().UTC()
	userID := insertDeletionUser(t, pool, userIDEmail("delete-me"), "Visible Name", "vis_"+uuid.NewString()[:8])
	otherID := insertDeletionUser(t, pool, userIDEmail("other"), "Other", "oth_"+uuid.NewString()[:8])

	network := "test-albatross"
	address := "NQ-DEL-" + userID.String()
	publicKey := make([]byte, ed25519.PublicKeySize)
	publicKey[0] = 1
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_nimiq_identities (user_id, network, address, public_key, verified_at, last_verified_at)
		VALUES ($1, $2, $3, $4, $5, $5)`, userID, network, address, publicKey, now); err != nil {
		t.Fatalf("insert identity: %v", err)
	}
	challengeID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO auth_challenges (id, nonce, purpose, transport, environment, network, domain, audience, claimed_address, message, issued_at, expires_at)
		VALUES ($1, $2, 'AUTH_LOGIN', 'mini-app', 'testnet', $3, 'nimnear.local', 'nimnear-api', $4, 'challenge', $5, $6)`,
		challengeID, []byte("nonce-"+userID.String()[:8]), network, address, now, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("insert challenge: %v", err)
	}

	calendarID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO calendars (id, name, owner_id, visibility, status) VALUES ($1, 'Public calendar', $2, 'public', 'active')`, calendarID, userID); err != nil {
		t.Fatalf("insert calendar: %v", err)
	}
	futureEventID := uuid.New()
	pastEventID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO events (id, title, description, starts_at, ends_at, status, price_lunas, currency, city, organizer_id, is_public)
		VALUES ($1, 'Future event', '', $2, $3, 'published', 0, 'NIM', 'Test City', $4, TRUE)`,
		futureEventID, now.Add(2*time.Hour), now.Add(3*time.Hour), userID); err != nil {
		t.Fatalf("insert future event: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO events (id, title, description, starts_at, ends_at, status, price_lunas, currency, city, organizer_id, is_public)
		VALUES ($1, 'Past event', '', $2, $3, 'published', 100000, 'NIM', 'Test City', $4, TRUE)`,
		pastEventID, now.Add(-3*time.Hour), now.Add(-2*time.Hour), userID); err != nil {
		t.Fatalf("insert past event: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO event_participants (event_id, user_id) VALUES ($1, $2)`, pastEventID, userID); err != nil {
		t.Fatalf("insert rsvp: %v", err)
	}
	purchaseID := uuid.New()
	txHash := compactHash("aa", uuid.NewString())
	if _, err := pool.Exec(ctx, `
		INSERT INTO event_purchases (id, event_id, user_id, amount_lunas, status, transaction_hash, confirmed_at)
		VALUES ($1, $2, $3, 100000, 'confirmed', $4, $5)`, purchaseID, pastEventID, userID, txHash, now); err != nil {
		t.Fatalf("insert purchase: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO consumed_nimiq_transactions (transaction_hash, domain_type, domain_id)
		VALUES ($1, 'event_purchase', $2)`, txHash, purchaseID); err != nil {
		t.Fatalf("insert consumed hash: %v", err)
	}

	pendingRequestID, pendingPublicID := uuid.New(), uuid.New()
	paidRequestID, paidPublicID := uuid.New(), uuid.New()
	paidHash := compactHash("bb", uuid.NewString())
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment_requests (id, public_id, creator_user_id, recipient_address, amount_lunas, status, expires_at)
		VALUES ($1, $2, $3, $4, 100000, 'pending', $5)`, pendingRequestID, pendingPublicID, userID, address, now.Add(24*time.Hour)); err != nil {
		t.Fatalf("insert pending request: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment_requests (id, public_id, creator_user_id, recipient_address, amount_lunas, status, expires_at, paid_at, payer_user_id, transaction_hash)
		VALUES ($1, $2, $3, $4, 100000, 'paid', $5, $6, $7, $8)`,
		paidRequestID, paidPublicID, userID, address, now.Add(24*time.Hour), now, otherID, paidHash); err != nil {
		t.Fatalf("insert paid request: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO consumed_nimiq_transactions (transaction_hash, domain_type, domain_id)
		VALUES ($1, 'payment_request', $2)`, paidHash, paidRequestID); err != nil {
		t.Fatalf("insert paid consumed hash: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM consumed_nimiq_transactions WHERE transaction_hash IN ($1, $2)", txHash, paidHash)
		_, _ = pool.Exec(ctx, "DELETE FROM payment_requests WHERE id IN ($1, $2)", pendingRequestID, paidRequestID)
		_, _ = pool.Exec(ctx, "DELETE FROM event_purchases WHERE id = $1", purchaseID)
		_, _ = pool.Exec(ctx, "DELETE FROM events WHERE id IN ($1, $2)", futureEventID, pastEventID)
		_, _ = pool.Exec(ctx, "DELETE FROM calendars WHERE id = $1", calendarID)
		_, _ = pool.Exec(ctx, "DELETE FROM auth_challenges WHERE claimed_address = $1", address)
		_, _ = pool.Exec(ctx, "DELETE FROM user_nimiq_identities WHERE user_id = $1", userID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1, $2)", userID, otherID)
	})

	jwt := infraAuth.NewJWTService(config.JWTConfig{Secret: "account-deletion-test-secret-32chars!!", ExpirationHours: 24, Issuer: "nimnear-test"})
	token, err := jwt.GenerateToken(ctx, service.TokenClaims{UserID: userID, Email: "delete-me@example.com"})
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	uc := usecase.NewDeleteAccountUseCase(NewAccountDeletionRepo(pool))
	if err := uc.Execute(ctx, userID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := uc.Execute(ctx, userID); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}

	users := NewUserRepo(pool)
	deleted, err := users.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("get deleted user: %v", err)
	}
	if deleted.IsActive() || !deleted.IsDeleted() || deleted.Email != "" || deleted.FirstName != "" || deleted.PasswordHash != "" {
		t.Fatalf("user was not anonymized: %#v", deleted)
	}
	var username *string
	var displayName *string
	var bio string
	if err := pool.QueryRow(ctx, "SELECT username, display_name, bio FROM users WHERE id = $1", userID).Scan(&username, &displayName, &bio); err != nil {
		t.Fatalf("pii: %v", err)
	}
	if username != nil || displayName != nil || bio != "" {
		t.Fatalf("profile PII remained username=%v display=%v bio=%q", username, displayName, bio)
	}
	if _, err := profile.NewProfileRepo(pool).GetPublic(ctx, userID); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("deleted profile remained public: %v", err)
	}

	claims, err := jwt.ValidateToken(ctx, token)
	if err != nil {
		t.Fatalf("old jwt still cryptographically valid: %v", err)
	}
	lookedUp, err := users.GetByID(ctx, claims.UserID)
	if err != nil || lookedUp.IsActive() {
		t.Fatalf("deleted user lookup must fail active check: user=%#v err=%v", lookedUp, err)
	}

	authRepo := NewNimiqAuthRepo(pool)
	fresh := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO auth_challenges (id, nonce, purpose, transport, environment, network, domain, audience, claimed_address, message, issued_at, expires_at)
		VALUES ($1, $2, 'AUTH_LOGIN', 'mini-app', 'testnet', $3, 'nimnear.local', 'nimnear-api', $4, 'new', $5, $6)`,
		fresh, []byte("nonce-"+fresh.String()[:8]), network, address, now, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("insert post-delete challenge: %v", err)
	}
	_, resolveErr := authRepo.ConsumeAndResolveIdentity(ctx, fresh, publicKey, now.Add(time.Minute), "")
	if resolveErr == nil || domainErr.ErrorCode(resolveErr) != "account_unavailable" {
		t.Fatalf("revoked identity resurrected account: %v", resolveErr)
	}

	var calendarStatus string
	if err := pool.QueryRow(ctx, "SELECT status FROM calendars WHERE id = $1", calendarID).Scan(&calendarStatus); err != nil || calendarStatus != "archived" {
		t.Fatalf("calendar status = %q err=%v", calendarStatus, err)
	}
	var futureStatus, pastStatus string
	if err := pool.QueryRow(ctx, "SELECT status FROM events WHERE id = $1", futureEventID).Scan(&futureStatus); err != nil || futureStatus != "cancelled" {
		t.Fatalf("future event status = %q err=%v", futureStatus, err)
	}
	if err := pool.QueryRow(ctx, "SELECT status FROM events WHERE id = $1", pastEventID).Scan(&pastStatus); err != nil || pastStatus != "published" {
		t.Fatalf("past event status = %q err=%v", pastStatus, err)
	}
	var organizer uuid.UUID
	if err := pool.QueryRow(ctx, "SELECT organizer_id FROM events WHERE id = $1", pastEventID).Scan(&organizer); err != nil || organizer != userID {
		t.Fatalf("organizer was rewritten: %s err=%v", organizer, err)
	}
	var rsvpCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM event_participants WHERE event_id = $1 AND user_id = $2", pastEventID, userID).Scan(&rsvpCount); err != nil || rsvpCount != 1 {
		t.Fatalf("rsvp lost: count=%d err=%v", rsvpCount, err)
	}
	var purchaseStatus, storedHash string
	if err := pool.QueryRow(ctx, "SELECT status, transaction_hash FROM event_purchases WHERE id = $1", purchaseID).Scan(&purchaseStatus, &storedHash); err != nil || purchaseStatus != "confirmed" || storedHash != txHash {
		t.Fatalf("purchase evidence lost: status=%q hash=%q err=%v", purchaseStatus, storedHash, err)
	}
	var pendingStatus, paidStatus string
	if err := pool.QueryRow(ctx, "SELECT status FROM payment_requests WHERE id = $1", pendingRequestID).Scan(&pendingStatus); err != nil || pendingStatus != "cancelled" {
		t.Fatalf("pending request status = %q err=%v", pendingStatus, err)
	}
	if err := pool.QueryRow(ctx, "SELECT status FROM payment_requests WHERE id = $1", paidRequestID).Scan(&paidStatus); err != nil || paidStatus != "paid" {
		t.Fatalf("paid request status = %q err=%v", paidStatus, err)
	}
	var consumed int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM consumed_nimiq_transactions WHERE transaction_hash IN ($1, $2)", txHash, paidHash).Scan(&consumed); err != nil || consumed != 2 {
		t.Fatalf("consumed hashes = %d err=%v", consumed, err)
	}
}

func TestDeletedUserCannotAuthenticateNormally(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	userID := insertDeletionUser(t, pool, userIDEmail("login-deleted"), "Name", "login_"+uuid.NewString()[:8])
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID) })
	email := ""
	if err := pool.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", userID).Scan(&email); err != nil {
		t.Fatalf("read email: %v", err)
	}
	if err := NewAccountDeletionRepo(pool).DeleteAccount(ctx, userID, time.Now().UTC()); err != nil {
		t.Fatalf("delete: %v", err)
	}
	user, err := NewUserRepo(pool).GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if user.IsActive() {
		t.Fatal("deleted user remained active")
	}
	if _, err := NewUserRepo(pool).GetByEmail(ctx, email); err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("anonymized email remained findable: %v", err)
	}
}

func insertDeletionUser(t *testing.T, pool *pgxpool.Pool, email, displayName, username string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, email, first_name, last_name, display_name, username, bio, avatar_url, password_hash, status)
		VALUES ($1, $2, $3, '', $3, $4, 'personal bio', 'https://example.com/a.png', 'hash', 'active')`,
		id, email, displayName, username); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func userIDEmail(prefix string) string {
	return prefix + "-" + uuid.NewString() + "@deletion-test.local"
}

func compactHash(prefix, raw string) string {
	value := prefix + raw
	for len(value) < 64 {
		value += "0"
	}
	return value[:64]
}
