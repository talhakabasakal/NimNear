package eventpurchase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// PurchaseRepo implements atomic paid-event purchase creation and server-owned payment transitions.
type PurchaseRepo struct {
	db *pgxpool.Pool
}

// NewPurchaseRepo creates a purchase repository.
func NewPurchaseRepo(db *pgxpool.Pool) *PurchaseRepo {
	return &PurchaseRepo{db: db}
}

// Create locks the public event, lazily expires old pending holds, then checks capacity and inserts idempotently.
func (r *PurchaseRepo) Create(ctx context.Context, eventID, userID uuid.UUID, now time.Time, holdDuration time.Duration) (*model.Purchase, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin purchase transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var priceLunas int64
	var capacity *int
	var attendeeCount int
	var isPast bool
	if err := tx.QueryRow(ctx, `
        SELECT price_lunas, capacity, attendee_count, ends_at <= $2
        FROM events
        WHERE id = $1 AND is_public = TRUE AND status = 'published'
        FOR UPDATE`, eventID, now).Scan(&priceLunas, &capacity, &attendeeCount, &isPast); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "event not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock event for purchase", err)
	}
	if isPast {
		return nil, domainErr.NewWithCode(domainErr.ErrEventPast, "event_past", "event has already ended", nil)
	}
	if priceLunas <= 0 {
		return nil, domainErr.NewWithCode(domainErr.ErrValidation, "paid_purchase_required", "purchases are available only for paid events", nil)
	}
	if capacity != nil && attendeeCount >= *capacity {
		return nil, domainErr.New(domainErr.ErrSoldOut, "event is sold out", nil)
	}

	if _, err := tx.Exec(ctx, `
        UPDATE event_purchases
        SET status = 'expired', updated_at = $2
        WHERE event_id = $1
          AND status = 'pending'
          AND capacity_hold_expires_at IS NOT NULL
          AND capacity_hold_expires_at <= $2`, eventID, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to expire purchase holds", err)
	}

	var existing model.Purchase
	err = tx.QueryRow(ctx, `
        SELECT id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
               reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
               transaction_hash, created_at, updated_at, confirmed_at
        FROM event_purchases
        WHERE event_id = $1 AND user_id = $2
          AND status IN ('pending', 'submitted', 'verifying', 'confirmed')
        ORDER BY created_at DESC
        LIMIT 1`, eventID, userID).Scan(purchaseArgs(&existing)...)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to commit existing purchase", err)
		}
		return &existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to check active purchase", err)
	}

	if capacity != nil {
		var count int
		if err := tx.QueryRow(ctx, `
            SELECT COUNT(*)::int
            FROM (
                SELECT user_id FROM event_participants WHERE event_id = $1
                UNION
                SELECT user_id FROM event_purchases
                WHERE event_id = $1 AND status IN ('confirmed', 'submitted', 'verifying')
                UNION
                SELECT user_id FROM event_purchases
                WHERE event_id = $1 AND status = 'pending'
                  AND (capacity_hold_expires_at IS NULL OR capacity_hold_expires_at > $2)
            ) AS active_attendees`, eventID, now).Scan(&count); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to check event capacity", err)
		}
		if count >= *capacity {
			return nil, domainErr.New(domainErr.ErrSoldOut, "event is sold out", nil)
		}
	}

	var holdExpiresAt *time.Time
	if capacity != nil {
		expires := now.Add(holdDuration)
		holdExpiresAt = &expires
	}
	var created model.Purchase
	if err := tx.QueryRow(ctx, `
        INSERT INTO event_purchases (
            event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
            created_at, updated_at
        ) VALUES ($1, $2, $3, 'pending', $4, $5, $5)
        RETURNING id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
                  reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
                  transaction_hash, created_at, updated_at, confirmed_at`,
		eventID, userID, priceLunas, holdExpiresAt, now).Scan(purchaseArgs(&created)...); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create purchase", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit purchase", err)
	}
	return &created, nil
}

// GetOwned returns a purchase only for its owner. Missing and foreign records are indistinguishable.
func (r *PurchaseRepo) GetOwned(ctx context.Context, purchaseID, userID uuid.UUID) (*model.Purchase, error) {
	var purchase model.Purchase
	err := r.db.QueryRow(ctx, `
        SELECT id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
               reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
               transaction_hash, created_at, updated_at, confirmed_at
        FROM event_purchases
        WHERE id = $1 AND user_id = $2`, purchaseID, userID).Scan(purchaseArgs(&purchase)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "purchase not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get purchase", err)
	}
	return &purchase, nil
}

// GetActiveForEvent returns the owner's active purchase for refresh/recovery.
func (r *PurchaseRepo) GetActiveForEvent(ctx context.Context, eventID, userID uuid.UUID) (*model.Purchase, error) {
	_, err := r.db.Exec(ctx, `
        UPDATE event_purchases
        SET status = 'expired', updated_at = NOW()
        WHERE event_id = $1 AND user_id = $2
          AND status = 'pending'
          AND capacity_hold_expires_at IS NOT NULL
          AND capacity_hold_expires_at <= NOW()`, eventID, userID)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to expire purchase hold", err)
	}

	var purchase model.Purchase
	err = r.db.QueryRow(ctx, `
        SELECT id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
               reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
               transaction_hash, created_at, updated_at, confirmed_at
        FROM event_purchases
        WHERE event_id = $1 AND user_id = $2
          AND status IN ('pending', 'submitted', 'verifying', 'confirmed')
        ORDER BY created_at DESC
        LIMIT 1`, eventID, userID).Scan(purchaseArgs(&purchase)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "purchase not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get active purchase", err)
	}
	return &purchase, nil
}

// SubmitTransaction persists only a validated hash and pins capacity while verification is pending.
func (r *PurchaseRepo) SubmitTransaction(ctx context.Context, purchaseID, userID uuid.UUID, transactionHash string, now, reconciliationDeadline time.Time) (*model.Purchase, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin transaction submission", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current model.Purchase
	err = tx.QueryRow(ctx, `
        SELECT id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
               reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
               transaction_hash, created_at, updated_at, confirmed_at
        FROM event_purchases
        WHERE id = $1 AND user_id = $2
        FOR UPDATE`, purchaseID, userID).Scan(purchaseArgs(&current)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "purchase not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock purchase", err)
	}

	if current.TransactionHash != nil && *current.TransactionHash == transactionHash &&
		(current.Status == model.StatusSubmitted || current.Status == model.StatusVerifying || current.Status == model.StatusConfirmed) {
		if err := tx.Commit(ctx); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to commit idempotent transaction submission", err)
		}
		return &current, nil
	}
	if current.Status != model.StatusPending {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "purchase_not_submittable", "purchase is not waiting for a transaction", nil)
	}

	var submitted model.Purchase
	if err := tx.QueryRow(ctx, `
        UPDATE event_purchases
        SET status = 'submitted', transaction_hash = $3,
            capacity_hold_expires_at = NULL, reconciliation_deadline_at = $5,
            last_verification_attempt_at = NULL, verification_claimed_until = NULL,
            updated_at = $4
        WHERE id = $1 AND user_id = $2 AND status = 'pending'
        RETURNING id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
                  reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
                  transaction_hash, created_at, updated_at, confirmed_at`,
		purchaseID, userID, transactionHash, now, reconciliationDeadline).Scan(purchaseArgs(&submitted)...); err != nil {
		if isUniqueViolation(err) {
			return nil, domainErr.NewWithCode(domainErr.ErrAlreadyExists, "transaction_hash_used", "transaction hash is already assigned to another purchase", err)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to submit transaction", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit transaction submission", err)
	}
	return &submitted, nil
}

// ClaimVerification atomically claims one verification attempt. The claim and retry
// timestamps prevent concurrent requests and workers from repeatedly calling the RPC.
func (r *PurchaseRepo) ClaimVerification(ctx context.Context, purchaseID, userID uuid.UUID, now time.Time, retryInterval, claimDuration time.Duration, force bool) (*model.Purchase, error) {
	if retryInterval < 0 {
		retryInterval = 0
	}
	if claimDuration < 30*time.Second {
		claimDuration = 30 * time.Second
	}
	threshold := now.Add(-retryInterval)
	var purchase model.Purchase
	err := r.db.QueryRow(ctx, `
        UPDATE event_purchases
        SET last_verification_attempt_at = $3,
            verification_claimed_until = $3 + ($6 * INTERVAL '1 second')
        WHERE id = $1 AND user_id = $2
          AND status IN ('submitted', 'verifying')
          AND (verification_claimed_until IS NULL OR verification_claimed_until <= $3)
          AND ($5 OR last_verification_attempt_at IS NULL OR last_verification_attempt_at <= $4)
        RETURNING id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
                  reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
                  transaction_hash, created_at, updated_at, confirmed_at`,
		purchaseID, userID, now, threshold, force, int64(claimDuration/time.Second)).Scan(purchaseArgs(&purchase)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to claim payment verification", err)
	}
	return &purchase, nil
}

// ListReconciliationCandidates returns a deterministic, bounded batch of unresolved purchases.
func (r *PurchaseRepo) ListReconciliationCandidates(ctx context.Context, now time.Time, retryInterval, fallbackDeadline time.Duration, limit int) ([]*model.Purchase, error) {
	if limit <= 0 {
		return nil, nil
	}
	if retryInterval < 0 {
		retryInterval = 0
	}
	if fallbackDeadline <= 0 {
		fallbackDeadline = time.Hour
	}
	rows, err := r.db.Query(ctx, `
        SELECT id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
               reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
               transaction_hash, created_at, updated_at, confirmed_at
        FROM event_purchases
        WHERE status IN ('submitted', 'verifying')
          AND (verification_claimed_until IS NULL OR verification_claimed_until <= $1)
          AND (
              COALESCE(reconciliation_deadline_at, updated_at + ($3 * INTERVAL '1 second')) <= $1
              OR last_verification_attempt_at IS NULL
              OR last_verification_attempt_at <= $2
          )
        ORDER BY CASE WHEN reconciliation_deadline_at IS NULL THEN 1 ELSE 0 END,
                 COALESCE(reconciliation_deadline_at, updated_at + ($3 * INTERVAL '1 second')),
                 id
        LIMIT $4`, now, now.Add(-retryInterval), int64(fallbackDeadline/time.Second), limit)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list payment reconciliation candidates", err)
	}
	defer rows.Close()
	purchases := make([]*model.Purchase, 0, limit)
	for rows.Next() {
		var purchase model.Purchase
		if err := rows.Scan(purchaseArgs(&purchase)...); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to read payment reconciliation candidate", err)
		}
		purchases = append(purchases, &purchase)
	}
	if err := rows.Err(); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to iterate payment reconciliation candidates", err)
	}
	return purchases, nil
}

// ExpireUnresolved releases an unresolved submitted/verifying purchase only after its
// reconciliation deadline. It cannot affect a terminal or confirmed purchase.
func (r *PurchaseRepo) ExpireUnresolved(ctx context.Context, purchaseID, userID uuid.UUID, now time.Time, fallbackDeadline time.Duration) (*model.Purchase, error) {
	if fallbackDeadline <= 0 {
		fallbackDeadline = time.Hour
	}
	var purchase model.Purchase
	err := r.db.QueryRow(ctx, `
        UPDATE event_purchases
        SET status = 'expired', capacity_hold_expires_at = NULL,
            verification_claimed_until = NULL, updated_at = $3
        WHERE id = $1 AND user_id = $2
          AND status IN ('submitted', 'verifying')
          AND COALESCE(reconciliation_deadline_at, updated_at + ($4 * INTERVAL '1 second')) <= $3
        RETURNING id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
                  reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
                  transaction_hash, created_at, updated_at, confirmed_at`,
		purchaseID, userID, now, int64(fallbackDeadline/time.Second)).Scan(purchaseArgs(&purchase)...)
	if err == nil {
		return &purchase, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to expire unresolved purchase", err)
	}
	current, getErr := r.GetOwned(ctx, purchaseID, userID)
	if getErr != nil {
		return nil, getErr
	}
	return current, nil
}

// SetVerificationState applies only server-decided verification outcomes.
func (r *PurchaseRepo) SetVerificationState(ctx context.Context, purchaseID, userID uuid.UUID, status model.Status, now time.Time) (*model.Purchase, error) {
	if status != model.StatusVerifying && status != model.StatusConfirmed && status != model.StatusFailed {
		return nil, domainErr.New(domainErr.ErrBadRequest, "invalid verification state", nil)
	}

	var purchase model.Purchase
	err := r.db.QueryRow(ctx, `
        UPDATE event_purchases
        SET status = $3,
            capacity_hold_expires_at = NULL,
            verification_claimed_until = NULL,
            confirmed_at = CASE WHEN $3 = 'confirmed' THEN COALESCE(confirmed_at, $4) ELSE confirmed_at END,
            updated_at = $4
        WHERE id = $1 AND user_id = $2
          AND status IN ('submitted', 'verifying')
        RETURNING id, event_id, user_id, amount_lunas, status, capacity_hold_expires_at,
                  reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
                  transaction_hash, created_at, updated_at, confirmed_at`,
		purchaseID, userID, string(status), now).Scan(purchaseArgs(&purchase)...)
	if err == nil {
		return &purchase, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to update verification state", err)
	}

	current, getErr := r.GetOwned(ctx, purchaseID, userID)
	if getErr != nil {
		return nil, getErr
	}
	if current.Status == status || (current.Status == model.StatusConfirmed && status == model.StatusConfirmed) {
		return current, nil
	}
	if current.Status == model.StatusFailed || current.Status == model.StatusExpired || current.Status == model.StatusCancelled || current.Status == model.StatusConfirmed {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "purchase_state_terminal", "purchase is already in a terminal state", nil)
	}
	return nil, domainErr.NewWithCode(domainErr.ErrConflict, "purchase_state_changed", "purchase verification state changed", nil)
}

func purchaseArgs(p *model.Purchase) []any {
	return []any{
		&p.ID, &p.EventID, &p.UserID, &p.AmountLunas, &p.Status,
		&p.CapacityHoldExpiresAt, &p.ReconciliationDeadlineAt, &p.LastVerificationAttemptAt,
		&p.VerificationClaimedUntil, &p.TransactionHash, &p.CreatedAt, &p.UpdatedAt, &p.ConfirmedAt,
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var _ repository.PurchaseRepository = (*PurchaseRepo)(nil)
