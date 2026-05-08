package db

import (
	"context"
	"database/sql"
)

func EnsureCatProfileSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS cat_profile (
			id SMALLINT PRIMARY KEY DEFAULT 1,
			display_name TEXT NOT NULL DEFAULT 'Charmaine Cat',
			tagline TEXT NOT NULL DEFAULT 'Charmaine''s personal assistant',
			bio TEXT NOT NULL DEFAULT '',
			avatar_photo_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT one_cat_profile CHECK (id = 1)
		);`,

		`CREATE TABLE IF NOT EXISTS cat_profile_stories (
			id BIGSERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			sort_order INT NOT NULL DEFAULT 0,
			is_published BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE TABLE IF NOT EXISTS cat_profile_photos (
			id BIGSERIAL PRIMARY KEY,
			filename TEXT UNIQUE NOT NULL,
			original_filename TEXT NOT NULL,
			content_type TEXT NOT NULL,
			file_path TEXT NOT NULL,
			public_url TEXT NOT NULL,
			caption TEXT NOT NULL DEFAULT '',
			alt_text TEXT NOT NULL DEFAULT '',
			sort_order INT NOT NULL DEFAULT 0,
			is_published BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'fk_cat_profile_avatar'
			) THEN
				ALTER TABLE cat_profile
				ADD CONSTRAINT fk_cat_profile_avatar
				FOREIGN KEY (avatar_photo_id)
				REFERENCES cat_profile_photos(id)
				ON DELETE SET NULL;
			END IF;
		END $$;`,

		`INSERT INTO cat_profile (id, display_name, tagline, bio)
		 VALUES (
			1,
			'Charmaine Cat',
			'Charmaine''s personal assistant',
			'Charmaine Cat is Charmaine''s AI personal assistant. She helps recruiters, HR teams, and hiring managers understand Charmaine''s background, projects, skills, and work preferences using Charmaine''s curated knowledge base.'
		 )
		 ON CONFLICT (id) DO NOTHING;`,

		`CREATE INDEX IF NOT EXISTS idx_cat_profile_stories_sort_order
			ON cat_profile_stories(sort_order, id);`,

		`CREATE INDEX IF NOT EXISTS idx_cat_profile_photos_sort_order
			ON cat_profile_photos(sort_order, id);`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
