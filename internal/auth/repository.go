package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

var ErrUserNotFound = errors.New("user not found")
var ErrSessionNotFound = errors.New("session not found")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindUserByUsername(ctx context.Context, username string) (*User, error) {
	return r.findUser(ctx, `
SELECT
	id, username, display_name, email, role, auth_provider, password_hash, google_sub,
	status, created_at, updated_at, last_login_at, last_seen_at, expires_at, invited_by_user_id, notes
FROM users
WHERE username = $1
LIMIT 1
`, username)
}

func (r *Repository) FindUserByID(ctx context.Context, id int64) (*User, error) {
	return r.findUser(ctx, `
SELECT
	id, username, display_name, email, role, auth_provider, password_hash, google_sub,
	status, created_at, updated_at, last_login_at, last_seen_at, expires_at, invited_by_user_id, notes
FROM users
WHERE id = $1
LIMIT 1
`, id)
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	return r.findUser(ctx, `
SELECT
	id, username, display_name, email, role, auth_provider, password_hash, google_sub,
	status, created_at, updated_at, last_login_at, last_seen_at, expires_at, invited_by_user_id, notes
FROM users
WHERE email = $1
LIMIT 1
`, email)
}

func (r *Repository) FindUserByGoogleSub(ctx context.Context, sub string) (*User, error) {
	return r.findUser(ctx, `
SELECT
	id, username, display_name, email, role, auth_provider, password_hash, google_sub,
	status, created_at, updated_at, last_login_at, last_seen_at, expires_at, invited_by_user_id, notes
FROM users
WHERE google_sub = $1
LIMIT 1
`, sub)
}

func (r *Repository) findUser(ctx context.Context, query string, arg any) (*User, error) {
	row := r.db.QueryRowContext(ctx, query, arg)

	var u User
	err := row.Scan(
		&u.ID,
		&u.Username,
		&u.DisplayName,
		&u.Email,
		&u.Role,
		&u.AuthProvider,
		&u.PasswordHash,
		&u.GoogleSub,
		&u.Status,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.LastLoginAt,
		&u.LastSeenAt,
		&u.ExpiresAt,
		&u.InvitedByUserID,
		&u.Notes,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &u, nil
}

func (r *Repository) CreateUser(ctx context.Context, u *User) (int64, error) {
	row := r.db.QueryRowContext(ctx, `
INSERT INTO users (
	username, display_name, email, role, auth_provider, password_hash, google_sub,
	status, expires_at, invited_by_user_id, notes
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id
`,
		u.Username,
		u.DisplayName,
		u.Email,
		u.Role,
		u.AuthProvider,
		u.PasswordHash,
		u.GoogleSub,
		u.Status,
		u.ExpiresAt,
		u.InvitedByUserID,
		u.Notes,
	)

	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) LinkGoogleAccount(ctx context.Context, userID int64, googleSub, email string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE users
SET google_sub = $2, email = COALESCE(email, $3), auth_provider = 'google', updated_at = NOW()
WHERE id = $1
`, userID, googleSub, email)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO oauth_accounts (user_id, provider, provider_sub, email)
VALUES ($1, 'google', $2, $3)
ON CONFLICT (provider, provider_sub)
DO UPDATE SET
	email = EXCLUDED.email,
	updated_at = NOW()
`, userID, googleSub, email)
	return err
}

func (r *Repository) UpdateUserLoginTimestamps(ctx context.Context, userID int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE users
SET last_login_at = $2, last_seen_at = $2, updated_at = NOW()
WHERE id = $1
`, userID, at)
	return err
}

func (r *Repository) UpdateUserLastSeen(ctx context.Context, userID int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE users
SET last_seen_at = $2, updated_at = NOW()
WHERE id = $1
`, userID, at)
	return err
}

func (r *Repository) CreateSession(ctx context.Context, userID int64, rawToken string, expiresAt time.Time, ipAddress, userAgent *string) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO user_sessions (
	user_id, session_token_hash, expires_at, ip_address, user_agent
)
VALUES ($1,$2,$3,$4,$5)
`,
		userID,
		hashToken(rawToken),
		expiresAt,
		ipAddress,
		userAgent,
	)
	return err
}

func (r *Repository) FindSessionByToken(ctx context.Context, rawToken string) (*Session, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
	id, user_id, session_token_hash, created_at, last_seen_at, expires_at, ip_address, user_agent, revoked_at
FROM user_sessions
WHERE session_token_hash = $1
LIMIT 1
`, hashToken(rawToken))

	var s Session
	err := row.Scan(
		&s.ID,
		&s.UserID,
		&s.SessionTokenHash,
		&s.CreatedAt,
		&s.LastSeenAt,
		&s.ExpiresAt,
		&s.IPAddress,
		&s.UserAgent,
		&s.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	return &s, nil
}

func (r *Repository) UpdateSessionLastSeen(ctx context.Context, sessionID int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE user_sessions
SET last_seen_at = $2
WHERE id = $1
`, sessionID, at)
	return err
}

func (r *Repository) RevokeSession(ctx context.Context, rawToken string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE user_sessions
SET revoked_at = $2
WHERE session_token_hash = $1
`, hashToken(rawToken), at)
	return err
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
