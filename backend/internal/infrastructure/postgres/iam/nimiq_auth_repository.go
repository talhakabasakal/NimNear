package iam

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type NimiqAuthRepo struct{ db *pgxpool.Pool }

func NewNimiqAuthRepo(db *pgxpool.Pool) *NimiqAuthRepo { return &NimiqAuthRepo{db: db} }

func (r *NimiqAuthRepo) CreateChallenge(ctx context.Context, c *usecase.NimiqChallenge) error {
	_, err := r.db.Exec(ctx, `INSERT INTO auth_challenges (id, nonce, purpose, transport, environment, network, domain, audience, claimed_address, message, issued_at, expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, c.ID, c.Nonce, c.Purpose, c.Transport, c.Environment, c.Network, c.Domain, c.Audience, c.ClaimedAddress, c.Message, c.IssuedAt, c.ExpiresAt)
	if err != nil {
		return domainErr.New(domainErr.ErrInternal, "failed to store authentication challenge", err)
	}
	return nil
}

func (r *NimiqAuthRepo) GetChallenge(ctx context.Context, id uuid.UUID) (*usecase.NimiqChallenge, error) {
	var c usecase.NimiqChallenge
	err := r.db.QueryRow(ctx, `SELECT id, nonce, purpose, transport, environment, network, domain, audience, claimed_address, message, issued_at, expires_at, consumed_at FROM auth_challenges WHERE id=$1`, id).Scan(&c.ID, &c.Nonce, &c.Purpose, &c.Transport, &c.Environment, &c.Network, &c.Domain, &c.Audience, &c.ClaimedAddress, &c.Message, &c.IssuedAt, &c.ExpiresAt, &c.ConsumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.NewWithCode(domainErr.ErrNotFound, "challenge_not_found", "authentication challenge not found", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to load authentication challenge", err)
	}
	return &c, nil
}

func (r *NimiqAuthRepo) ConsumeAndResolveIdentity(ctx context.Context, challengeID uuid.UUID, publicKey []byte, now time.Time, accountLabel string) (*model.User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to start authentication transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var network, address string
	err = tx.QueryRow(ctx, `UPDATE auth_challenges SET consumed_at=$2 WHERE id=$1 AND consumed_at IS NULL AND expires_at>$2 RETURNING network, claimed_address`, challengeID, now).Scan(&network, &address)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "challenge_already_used_or_expired", "authentication challenge was already used or expired", nil)
	}
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to consume authentication challenge", err)
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, network+":"+address); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to lock Nimiq identity", err)
	}
	user, err := scanNimiqUser(tx.QueryRow(ctx, `SELECT u.id,COALESCE(u.email,''),COALESCE(u.password_hash,''),COALESCE(u.first_name,''),COALESCE(u.last_name,''),u.status,u.deleted_at,u.created_at,u.updated_at,i.public_key,i.revoked_at FROM user_nimiq_identities i JOIN users u ON u.id=i.user_id WHERE i.network=$1 AND i.address=$2`, network, address), publicKey)
	if err == nil {
		if _, err = tx.Exec(ctx, `UPDATE user_nimiq_identities SET last_verified_at=$3 WHERE network=$1 AND address=$2 AND revoked_at IS NULL`, network, address, now); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to update Nimiq identity", err)
		}
		if displayName := usecase.DisplayNameForWallet(address, accountLabel); displayName != "" {
			if _, err = tx.Exec(ctx, `UPDATE users SET first_name = CASE WHEN NULLIF(BTRIM(COALESCE(first_name, '')), '') IS NULL THEN $2 ELSE first_name END, display_name = CASE WHEN NULLIF(BTRIM(COALESCE(display_name, '')), '') IS NULL THEN $2 ELSE display_name END, updated_at = $3 WHERE id = $1`, user.ID, displayName, now); err != nil {
				return nil, domainErr.New(domainErr.ErrInternal, "failed to bind Nimiq profile", err)
			}
			if strings.TrimSpace(user.FirstName) == "" {
				user.FirstName = displayName
			}
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to commit authentication", err)
		}
		return user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	displayName := usecase.DisplayNameForWallet(address, accountLabel)
	user = &model.User{ID: uuid.New(), FirstName: displayName, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now}
	if _, err = tx.Exec(ctx, `INSERT INTO users (id,email,password_hash,first_name,last_name,display_name,status,created_at,updated_at) VALUES ($1,NULL,NULL,$2,'',$2,$3,$4,$4)`, user.ID, displayName, user.Status, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to create wallet user", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_nimiq_identities (user_id,network,address,public_key,verified_at,last_verified_at) VALUES ($1,$2,$3,$4,$5,$5)`, user.ID, network, address, publicKey, now); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to bind Nimiq identity", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to commit authentication", err)
	}
	return user, nil
}

func (r *NimiqAuthRepo) AddressForUser(ctx context.Context, userID uuid.UUID) (string, error) {
	addresses, err := r.VerifiedAddresses(ctx, userID)
	if err != nil || len(addresses) == 0 {
		return "", err
	}
	return addresses[0], nil
}

func (r *NimiqAuthRepo) VerifiedAddresses(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT address FROM user_nimiq_identities WHERE user_id=$1 AND revoked_at IS NULL ORDER BY last_verified_at DESC`, userID)
	if err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to load Nimiq identity", err)
	}
	defer rows.Close()
	var addresses []string
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			return nil, domainErr.New(domainErr.ErrInternal, "failed to load Nimiq identity", err)
		}
		if strings.TrimSpace(address) != "" {
			addresses = append(addresses, address)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, domainErr.New(domainErr.ErrInternal, "failed to load Nimiq identity", err)
	}
	return addresses, nil
}

type rowScanner interface{ Scan(...any) error }

func scanNimiqUser(row rowScanner, expectedKey []byte) (*model.User, error) {
	var u model.User
	var storedKey []byte
	var revokedAt *time.Time
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Status, &u.DeletedAt, &u.CreatedAt, &u.UpdatedAt, &storedKey, &revokedAt)
	if err != nil {
		return nil, err
	}
	if revokedAt != nil || u.DeletedAt != nil || u.Status != model.UserStatusActive {
		return nil, domainErr.NewWithCode(domainErr.ErrForbidden, "account_unavailable", "this account is no longer available", nil)
	}
	if !equalBytes(storedKey, expectedKey) {
		return nil, domainErr.NewWithCode(domainErr.ErrConflict, "nimiq_identity_key_mismatch", "stored Nimiq identity key does not match", nil)
	}
	return &u, nil
}
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
