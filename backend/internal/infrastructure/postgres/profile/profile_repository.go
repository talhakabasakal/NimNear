package profile

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/masterfabric-go/masterfabric/internal/domain/profile/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// ProfileRepo reads public profile data and updates the authenticated user's profile.
type ProfileRepo struct {
	db *pgxpool.Pool
}

// NewProfileRepo creates a PostgreSQL profile repository.
func NewProfileRepo(db *pgxpool.Pool) *ProfileRepo {
	return &ProfileRepo{db: db}
}

// GetPublic returns only active users and public profile fields.
func (r *ProfileRepo) GetPublic(ctx context.Context, id uuid.UUID) (*model.PublicProfile, error) {
	var profile model.PublicProfile
	err := r.db.QueryRow(ctx, `
		SELECT
			u.id,
			COALESCE(
				NULLIF(BTRIM(u.display_name), ''),
				NULLIF(CONCAT_WS(' ', NULLIF(BTRIM(u.first_name), ''), NULLIF(BTRIM(u.last_name), '')), ''),
				i.address,
				''
			),
			u.username,
			u.bio,
			u.avatar_url,
			COALESCE(i.address, ''),
			u.created_at,
			(
				SELECT COUNT(*)::int
				FROM events e
				WHERE e.organizer_id = u.id
				  AND e.is_public = TRUE
				  AND e.status = 'published'
			),
			(
				SELECT COUNT(*)::int
				FROM (
					SELECT ep.event_id FROM event_participants ep WHERE ep.user_id = u.id
					UNION
					SELECT ep.event_id FROM event_purchases ep WHERE ep.user_id = u.id AND ep.status = 'confirmed'
				) attended_events
				JOIN events e ON e.id = attended_events.event_id
				WHERE e.is_public = TRUE AND e.status = 'published'
			)
		FROM users u
		LEFT JOIN user_nimiq_identities i ON i.user_id = u.id AND i.revoked_at IS NULL
		WHERE u.id = $1 AND u.status = 'active'`, id,
	).Scan(
		&profile.ID,
		&profile.DisplayName,
		&profile.Username,
		&profile.Bio,
		&profile.AvatarURL,
		&profile.WalletAddress,
		&profile.JoinedAt,
		&profile.OrganizedEventCount,
		&profile.AttendedEventCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "profile not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to get public profile", err)
	}
	return &profile, nil
}

// UpdateSelf updates only the explicitly supplied profile fields for one user.
func (r *ProfileRepo) UpdateSelf(ctx context.Context, id uuid.UUID, patch model.ProfilePatch) (*model.PublicProfile, error) {
	var updatedID uuid.UUID
	err := r.db.QueryRow(ctx, `
		UPDATE users
		SET
			display_name = CASE WHEN $2 THEN $3::text ELSE display_name END,
			username = CASE WHEN $4 THEN $5::varchar ELSE username END,
			bio = CASE WHEN $6 THEN COALESCE($7::text, '') ELSE bio END,
			updated_at = NOW()
		WHERE id = $1 AND status = 'active'
		RETURNING id`,
		id,
		patch.DisplayNameSet, patch.DisplayName,
		patch.UsernameSet, patch.Username,
		patch.BioSet, patch.Bio,
	).Scan(&updatedID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domainErr.NewWithCode(domainErr.ErrAlreadyExists, "username_taken", "username is already taken", nil)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainErr.New(domainErr.ErrNotFound, "profile not found", nil)
		}
		return nil, domainErr.New(domainErr.ErrInternal, "failed to update profile", err)
	}
	return r.GetPublic(ctx, updatedID)
}
