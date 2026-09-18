package paymentrequest

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/repository"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/nimiqtx"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const requestColumns = `id, public_id, creator_user_id, recipient_address, amount_lunas, note, status,
               expires_at, cancelled_at, paid_at, payer_user_id, transaction_hash,
               reconciliation_deadline_at, last_verification_attempt_at, verification_claimed_until,
               created_at, updated_at`

// RequestRepo implements atomic payment-request persistence and server-owned payment transitions.
type RequestRepo struct {
	db *pgxpool.Pool
}

func NewRequestRepo(db *pgxpool.Pool) *RequestRepo {
	return &RequestRepo{db: db}
}

func (r *RequestRepo) Create(ctx context.Context, request *model.Request) (*model.Request, error) {
	if request == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "payment request is required", nil)
	}
	var created model.Request
	err := r.db.QueryRow(ctx, `
        INSERT INTO payment_requests (
            id, public_id, creator_user_id, recipient_address, amount_lunas, note, status,
            expires_at, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8, $8)
        RETURNING `+requestColumns,
		request.ID, request.PublicID, request.CreatorUserID, request.RecipientAddress,
		request.AmountLunas, request.Note, request.ExpiresAt, request.CreatedAt,
	).Scan(requestArgs(&created)...)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create payment request", err)
	}
	return &created, nil
}

func (r *RequestRepo) GetByPublicID(ctx context.Context, publicID uuid.UUID, now time.Time) (*model.Request, error) {
	if err := r.expirePending(ctx, publicID, now); err != nil {
		return nil, err
	}
	var request model.Request
	err := r.db.QueryRow(ctx, `SELECT `+requestColumns+` FROM payment_requests WHERE public_id = $1`, publicID).Scan(requestArgs(&request)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get payment request", err)
	}
	return &request, nil
}

func (r *RequestRepo) GetOwnedByPublicID(ctx context.Context, publicID, creatorID uuid.UUID, now time.Time) (*model.Request, error) {
	if err := r.expirePending(ctx, publicID, now); err != nil {
		return nil, err
	}
	var request model.Request
	err := r.db.QueryRow(ctx, `SELECT `+requestColumns+` FROM payment_requests WHERE public_id = $1 AND creator_user_id = $2`, publicID, creatorID).Scan(requestArgs(&request)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get payment request", err)
	}
	return &request, nil
}

func (r *RequestRepo) ListByCreator(ctx context.Context, creatorID uuid.UUID, now time.Time, limit int) ([]*model.Request, error) {
	if _, err := r.db.Exec(ctx, `
        UPDATE payment_requests
        SET status = 'expired', updated_at = $2
        WHERE creator_user_id = $1 AND status = 'pending' AND expires_at <= $2`, creatorID, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to expire payment requests", err)
	}
	rows, err := r.db.Query(ctx, `
        SELECT `+requestColumns+`
        FROM payment_requests
        WHERE creator_user_id = $1
        ORDER BY created_at DESC, id DESC
        LIMIT $2`, creatorID, limit)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list payment requests", err)
	}
	defer rows.Close()
	requests := make([]*model.Request, 0, limit)
	for rows.Next() {
		var request model.Request
		if err := rows.Scan(requestArgs(&request)...); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to read payment request", err)
		}
		requests = append(requests, &request)
	}
	if err := rows.Err(); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to iterate payment requests", err)
	}
	return requests, nil
}

func (r *RequestRepo) Cancel(ctx context.Context, publicID, creatorID uuid.UUID, now time.Time) (*model.Request, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin payment request cancellation", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current model.Request
	err = tx.QueryRow(ctx, `
        SELECT `+requestColumns+`
        FROM payment_requests
        WHERE public_id = $1 AND creator_user_id = $2
        FOR UPDATE`, publicID, creatorID).Scan(requestArgs(&current)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock payment request", err)
	}

	if current.Status == model.StatusPending && !now.Before(current.ExpiresAt) {
		expired, err := expirePendingTx(ctx, tx, publicID, now)
		if err != nil {
			return nil, err
		}
		if expired != nil {
			current = *expired
		} else {
			current.Status = model.StatusExpired
			current.UpdatedAt = now
		}
	}

	switch current.Status {
	case model.StatusCancelled, model.StatusExpired:
		if err := tx.Commit(ctx); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to commit payment request cancellation", err)
		}
		return &current, nil
	case model.StatusPending:
		var cancelled model.Request
		if err := tx.QueryRow(ctx, `
            UPDATE payment_requests
            SET status = 'cancelled', cancelled_at = COALESCE(cancelled_at, $3), updated_at = $3
            WHERE public_id = $1 AND creator_user_id = $2 AND status = 'pending'
            RETURNING `+requestColumns, publicID, creatorID, now).Scan(requestArgs(&cancelled)...); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to cancel payment request", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to commit payment request cancellation", err)
		}
		return &cancelled, nil
	default:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_cancellable", "payment request can no longer be cancelled", nil)
	}
}

func (r *RequestRepo) SubmitTransaction(ctx context.Context, publicID, payerID uuid.UUID, transactionHash string, now, reconciliationDeadline time.Time) (*model.Request, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin payment request submission", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current model.Request
	err = tx.QueryRow(ctx, `
        SELECT `+requestColumns+`
        FROM payment_requests
        WHERE public_id = $1
        FOR UPDATE`, publicID).Scan(requestArgs(&current)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock payment request", err)
	}

	if current.Status == model.StatusPending && !now.Before(current.ExpiresAt) {
		expired, err := expirePendingTx(ctx, tx, publicID, now)
		if err != nil {
			return nil, err
		}
		if expired != nil {
			current = *expired
		} else {
			current.Status = model.StatusExpired
		}
	}

	if current.TransactionHash != nil && *current.TransactionHash == transactionHash &&
		current.PayerUserID != nil && *current.PayerUserID == payerID &&
		(current.Status == model.StatusSubmitted || current.Status == model.StatusVerifying || current.Status == model.StatusPaid) {
		if err := tx.Commit(ctx); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to commit idempotent payment request submission", err)
		}
		return &current, nil
	}

	switch current.Status {
	case model.StatusPaid:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_already_paid", "payment request is already paid", nil)
	case model.StatusCancelled:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_cancelled", "payment request is cancelled", nil)
	case model.StatusExpired:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_expired", "payment request has expired", nil)
	case model.StatusFailed, model.StatusSubmitted, model.StatusVerifying:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_submittable", "payment request is not waiting for a transaction", nil)
	case model.StatusPending:
	default:
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_submittable", "payment request is not waiting for a transaction", nil)
	}

	var submitted model.Request
	if err := tx.QueryRow(ctx, `
        UPDATE payment_requests
        SET status = 'submitted', transaction_hash = $2, payer_user_id = $3,
            reconciliation_deadline_at = $5, last_verification_attempt_at = NULL,
            verification_claimed_until = NULL, updated_at = $4
        WHERE id = $1 AND status = 'pending'
        RETURNING `+requestColumns, current.ID, transactionHash, payerID, now, reconciliationDeadline).Scan(requestArgs(&submitted)...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_submittable", "payment request is not waiting for a transaction", nil)
		}
		if isUniqueViolation(err) {
			return nil, domainErr.NewWithCode(domainErr.ErrAlreadyExists, "transaction_hash_used", "transaction hash is already consumed", err)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to submit payment request transaction", err)
	}
	if err := nimiqtx.Consume(ctx, tx, transactionHash, nimiqtx.DomainPaymentRequest, submitted.ID, now); err != nil {
		if isUniqueViolation(err) {
			return nil, domainErr.NewWithCode(domainErr.ErrAlreadyExists, "transaction_hash_used", "transaction hash is already consumed", err)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to consume transaction hash", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit payment request submission", err)
	}
	return &submitted, nil
}

func (r *RequestRepo) ClaimVerification(ctx context.Context, id uuid.UUID, now time.Time, retryInterval, claimDuration time.Duration, force bool) (*model.Request, error) {
	if retryInterval < 0 {
		retryInterval = 0
	}
	if claimDuration < 30*time.Second {
		claimDuration = 30 * time.Second
	}
	threshold := now.Add(-retryInterval)
	var request model.Request
	err := r.db.QueryRow(ctx, `
        UPDATE payment_requests
        SET last_verification_attempt_at = $2,
            verification_claimed_until = $2 + ($5 * INTERVAL '1 second')
        WHERE id = $1
          AND status IN ('submitted', 'verifying')
          AND (verification_claimed_until IS NULL OR verification_claimed_until <= $2)
          AND ($4 OR last_verification_attempt_at IS NULL OR last_verification_attempt_at <= $3)
        RETURNING `+requestColumns, id, now, threshold, force, int64(claimDuration/time.Second)).Scan(requestArgs(&request)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to claim payment request verification", err)
	}
	return &request, nil
}

func (r *RequestRepo) ListReconciliationCandidates(ctx context.Context, now time.Time, retryInterval, fallbackDeadline time.Duration, limit int) ([]*model.Request, error) {
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
        SELECT `+requestColumns+`
        FROM payment_requests
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
		return nil, domainErr.New(domainErr.ErrInternal, "failed to list payment request reconciliation candidates", err)
	}
	defer rows.Close()
	requests := make([]*model.Request, 0, limit)
	for rows.Next() {
		var request model.Request
		if err := rows.Scan(requestArgs(&request)...); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to read payment request reconciliation candidate", err)
		}
		requests = append(requests, &request)
	}
	if err := rows.Err(); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to iterate payment request reconciliation candidates", err)
	}
	return requests, nil
}

func (r *RequestRepo) ExpireUnresolved(ctx context.Context, id uuid.UUID, now time.Time, fallbackDeadline time.Duration) (*model.Request, error) {
	if fallbackDeadline <= 0 {
		fallbackDeadline = time.Hour
	}
	var request model.Request
	err := r.db.QueryRow(ctx, `
        UPDATE payment_requests
        SET status = 'expired', verification_claimed_until = NULL, updated_at = $2
        WHERE id = $1
          AND status IN ('submitted', 'verifying')
          AND COALESCE(reconciliation_deadline_at, updated_at + ($3 * INTERVAL '1 second')) <= $2
        RETURNING `+requestColumns, id, now, int64(fallbackDeadline/time.Second)).Scan(requestArgs(&request)...)
	if err == nil {
		return &request, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to expire unresolved payment request", err)
	}
	return r.getByID(ctx, id)
}

// ConfirmExpiredRecovery is the only path that may move expired → paid, and only after
// the shared verifier has already succeeded for this same consumed hash.
func (r *RequestRepo) ConfirmExpiredRecovery(ctx context.Context, id uuid.UUID, now time.Time) (*model.Request, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to begin expired payment request recovery", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current model.Request
	err = tx.QueryRow(ctx, `
        SELECT `+requestColumns+`
        FROM payment_requests
        WHERE id = $1
        FOR UPDATE`, id).Scan(requestArgs(&current)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock payment request for recovery", err)
	}
	if current.Status == model.StatusPaid {
		if err := tx.Commit(ctx); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to commit recovered payment request", err)
		}
		return &current, nil
	}
	if current.Status != model.StatusExpired || current.TransactionHash == nil || strings.TrimSpace(*current.TransactionHash) == "" {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_not_recoverable", "payment request is not an expired payment with a reserved transaction", nil)
	}

	var domainType string
	var domainID uuid.UUID
	err = tx.QueryRow(ctx, `
        SELECT domain_type, domain_id
        FROM consumed_nimiq_transactions
        WHERE transaction_hash = $1
        FOR UPDATE`, *current.TransactionHash).Scan(&domainType, &domainID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "transaction_not_reserved", "transaction hash is not reserved for this payment request", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock consumed transaction", err)
	}
	if domainType != nimiqtx.DomainPaymentRequest || domainID != current.ID {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "transaction_not_reserved", "transaction hash is not reserved for this payment request", nil)
	}

	var paid model.Request
	if err := tx.QueryRow(ctx, `
        UPDATE payment_requests
        SET status = 'paid', verification_claimed_until = NULL,
            paid_at = COALESCE(paid_at, $2), updated_at = $2
        WHERE id = $1 AND status = 'expired' AND transaction_hash IS NOT NULL
        RETURNING `+requestColumns, id, now).Scan(requestArgs(&paid)...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_state_changed", "payment request recovery state changed", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to confirm recovered payment request", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit recovered payment request", err)
	}
	return &paid, nil
}

func (r *RequestRepo) SetVerificationState(ctx context.Context, id uuid.UUID, status model.Status, now time.Time) (*model.Request, error) {
	if status != model.StatusVerifying && status != model.StatusPaid && status != model.StatusFailed {
		return nil, domainErr.New(domainErr.ErrBadRequest, "invalid verification state", nil)
	}
	var request model.Request
	err := r.db.QueryRow(ctx, `
        UPDATE payment_requests
        SET status = $2::text,
            verification_claimed_until = NULL,
            paid_at = CASE WHEN $2::text = 'paid' THEN COALESCE(paid_at, $3) ELSE paid_at END,
            updated_at = $3
        WHERE id = $1
          AND status IN ('submitted', 'verifying')
        RETURNING `+requestColumns, id, string(status), now).Scan(requestArgs(&request)...)
	if err == nil {
		return &request, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to update payment request verification state", err)
	}
	current, getErr := r.getByID(ctx, id)
	if getErr != nil {
		return nil, getErr
	}
	if current.Status == status {
		return current, nil
	}
	if current.IsTerminal() {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_state_terminal", "payment request is already in a terminal state", nil)
	}
	return nil, domainErr.NewWithCode(domainErr.ErrConflict, "payment_request_state_changed", "payment request verification state changed", nil)
}

func (r *RequestRepo) expirePending(ctx context.Context, publicID uuid.UUID, now time.Time) error {
	_, err := r.db.Exec(ctx, `
        UPDATE payment_requests
        SET status = 'expired', updated_at = $2
        WHERE public_id = $1 AND status = 'pending' AND expires_at <= $2`, publicID, now)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to expire payment request", err)
	}
	return nil
}

func expirePendingTx(ctx context.Context, tx pgx.Tx, publicID uuid.UUID, now time.Time) (*model.Request, error) {
	var request model.Request
	err := tx.QueryRow(ctx, `
        UPDATE payment_requests
        SET status = 'expired', updated_at = $2
        WHERE public_id = $1 AND status = 'pending' AND expires_at <= $2
        RETURNING `+requestColumns, publicID, now).Scan(requestArgs(&request)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to expire payment request", err)
	}
	return &request, nil
}

func (r *RequestRepo) getByID(ctx context.Context, id uuid.UUID) (*model.Request, error) {
	var request model.Request
	err := r.db.QueryRow(ctx, `SELECT `+requestColumns+` FROM payment_requests WHERE id = $1`, id).Scan(requestArgs(&request)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.New(domainErr.ErrNotFound, "payment request not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get payment request", err)
	}
	return &request, nil
}

func requestArgs(r *model.Request) []any {
	return []any{
		&r.ID, &r.PublicID, &r.CreatorUserID, &r.RecipientAddress, &r.AmountLunas, &r.Note, &r.Status,
		&r.ExpiresAt, &r.CancelledAt, &r.PaidAt, &r.PayerUserID, &r.TransactionHash,
		&r.ReconciliationDeadlineAt, &r.LastVerificationAttemptAt, &r.VerificationClaimedUntil,
		&r.CreatedAt, &r.UpdatedAt,
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var _ repository.RequestRepository = (*RequestRepo)(nil)
