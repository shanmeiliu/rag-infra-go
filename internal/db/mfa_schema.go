package db

import (
	"context"
	"database/sql"
)

func EnsureMFASchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`ALTER TABLE users
			ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE;`,

		`ALTER TABLE users
			ADD COLUMN IF NOT EXISTS mfa_totp_secret TEXT;`,

		`ALTER TABLE users
			ADD COLUMN IF NOT EXISTS mfa_confirmed_at TIMESTAMPTZ;`,

		`ALTER TABLE users
			ADD COLUMN IF NOT EXISTS mfa_email_enabled BOOLEAN NOT NULL DEFAULT FALSE;`,

		`ALTER TABLE users
			ADD COLUMN IF NOT EXISTS mfa_email TEXT;`,

		`CREATE TABLE IF NOT EXISTS mfa_challenges (
			id BIGSERIAL PRIMARY KEY,
			challenge_token_hash TEXT UNIQUE NOT NULL,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			email_code_hash TEXT,
			email_code_expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL,
			consumed_at TIMESTAMPTZ
		);`,

		`CREATE INDEX IF NOT EXISTS idx_mfa_challenges_user_id
			ON mfa_challenges(user_id);`,

		`CREATE INDEX IF NOT EXISTS idx_mfa_challenges_expires_at
			ON mfa_challenges(expires_at);`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
