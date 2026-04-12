package db

import (
	"context"
	"database/sql"
)

func EnsureAuthSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			display_name TEXT,
			email TEXT,
			role TEXT NOT NULL,
			auth_provider TEXT NOT NULL,
			password_hash TEXT,
			google_sub TEXT UNIQUE,
			status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_login_at TIMESTAMPTZ,
			last_seen_at TIMESTAMPTZ,
			expires_at TIMESTAMPTZ,
			invited_by_user_id BIGINT,
			notes TEXT
		);`,

		`CREATE TABLE IF NOT EXISTS user_sessions (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			session_token_hash TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL,
			ip_address TEXT,
			user_agent TEXT,
			revoked_at TIMESTAMPTZ
		);`,

		`CREATE TABLE IF NOT EXISTS oauth_accounts (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			provider_sub TEXT NOT NULL,
			email TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(provider, provider_sub)
		);`,

		`CREATE TABLE IF NOT EXISTS user_invites (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			invite_type TEXT NOT NULL,
			token_hash TEXT,
			code_hash TEXT,
			username TEXT,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_by_user_id BIGINT NOT NULL REFERENCES users(id),
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb
		);`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
			action TEXT NOT NULL,
			target_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
			details JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);`,
		`CREATE INDEX IF NOT EXISTS idx_users_auth_provider ON users(auth_provider);`,
		`CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);`,
		`CREATE INDEX IF NOT EXISTS idx_users_last_seen_at ON users(last_seen_at);`,
		`CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);`,
		`CREATE INDEX IF NOT EXISTS idx_user_invites_user_id ON user_invites(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_user_invites_expires_at ON user_invites(expires_at);`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
