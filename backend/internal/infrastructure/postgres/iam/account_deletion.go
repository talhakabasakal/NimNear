package iam

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// AccountDeletionRepo anonymizes a user while preserving financial evidence.
type AccountDeletionRepo struct {
	db *pgxpool.Pool
}

// NewAccountDeletionRepo creates an AccountDeletionRepo.
func NewAccountDeletionRepo(db *pgxpool.Pool) *AccountDeletionRepo {
	return &AccountDeletionRepo{db: db}
}

var _ repository.AccountDeletionRepository = (*AccountDeletionRepo)(nil)

// DeleteAccount deactivates the user, strips user-facing PII, revokes Nimiq
// identities, consumes outstanding auth challenges, archives owned calendars,
// cancels pending payment requests, and cancels unpublished future events.
// Payment evidence, RSVP rows, EventPurchase rows, and consumed transaction
// hashes are retained. Repeating the operation is a no-op.
func (r *AccountDeletionRepo) DeleteAccount(ctx context.Context, userID uuid.UUID, now time.Time) error {
	if userID == uuid.Nil {
		return domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil)
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to start account deletion", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	var deletedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT status, deleted_at FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&currentStatus, &deletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainErr.New(domainErr.ErrNotFound, "user not found", nil)
	}
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to lock user for deletion", err)
	}
	if deletedAt != nil {
		if err := tx.Commit(ctx); err != nil {
			return domainErr.New(domainErr.ErrInternal, "failed to commit account deletion", err)
		}
		return nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE auth_challenges
		SET consumed_at = $2
		WHERE consumed_at IS NULL
		  AND claimed_address IN (
		      SELECT address FROM user_nimiq_identities WHERE user_id = $1
		  )`, userID, now); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to invalidate authentication challenges", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE user_nimiq_identities
		SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL`, userID, now); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to revoke Nimiq identities", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE calendars
		SET status = 'archived', updated_at = $2
		WHERE owner_id = $1 AND status = 'active'`, userID, now); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to archive owned calendars", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE payment_requests
		SET status = 'cancelled', cancelled_at = COALESCE(cancelled_at, $2), updated_at = $2
		WHERE creator_user_id = $1 AND status = 'pending'`, userID, now); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to cancel pending payment requests", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE events
		SET status = 'cancelled', updated_at = $2
		WHERE organizer_id = $1
		  AND is_public = TRUE
		  AND status = 'published'
		  AND ends_at > $2`, userID, now); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to cancel future organized events", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET
			email = NULL,
			password_hash = NULL,
			first_name = '',
			last_name = '',
			display_name = NULL,
			username = NULL,
			bio = '',
			avatar_url = '',
			status = 'inactive',
			deleted_at = $2,
			updated_at = $2
		WHERE id = $1`, userID, now); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to anonymize user", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to commit account deletion", err)
	}
	return nil
}
