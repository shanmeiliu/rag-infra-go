package catprofile

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("cat profile record not found")

type Profile struct {
	DisplayName   string    `json:"display_name"`
	Tagline       string    `json:"tagline"`
	Bio           string    `json:"bio"`
	AvatarPhotoID *int64    `json:"avatar_photo_id,omitempty"`
	AvatarURL     *string   `json:"avatar_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Story struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Body         string    `json:"body"`
	PhotoID      *int64    `json:"photo_id,omitempty"`
	PhotoURL     *string   `json:"photo_url,omitempty"`
	PhotoCaption *string   `json:"photo_caption,omitempty"`
	PhotoAltText *string   `json:"photo_alt_text,omitempty"`
	SortOrder    int       `json:"sort_order"`
	IsPublished  bool      `json:"is_published"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Photo struct {
	ID               int64     `json:"id"`
	Filename         string    `json:"filename"`
	OriginalFilename string    `json:"original_filename"`
	ContentType      string    `json:"content_type"`
	FilePath         string    `json:"-"`
	PublicURL        string    `json:"public_url"`
	Caption          string    `json:"caption"`
	AltText          string    `json:"alt_text"`
	SortOrder        int       `json:"sort_order"`
	IsPublished      bool      `json:"is_published"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetProfile(ctx context.Context) (*Profile, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
	p.display_name,
	p.tagline,
	p.bio,
	p.avatar_photo_id,
	ph.public_url,
	p.created_at,
	p.updated_at
FROM cat_profile p
LEFT JOIN cat_profile_photos ph ON ph.id = p.avatar_photo_id
WHERE p.id = 1
`)

	var p Profile
	err := row.Scan(
		&p.DisplayName,
		&p.Tagline,
		&p.Bio,
		&p.AvatarPhotoID,
		&p.AvatarURL,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, displayName, tagline, bio string, avatarPhotoID *int64) (*Profile, error) {
	_, err := r.db.ExecContext(ctx, `
UPDATE cat_profile
SET
	display_name = $1,
	tagline = $2,
	bio = $3,
	avatar_photo_id = $4,
	updated_at = NOW()
WHERE id = 1
`, displayName, tagline, bio, avatarPhotoID)
	if err != nil {
		return nil, err
	}

	return r.GetProfile(ctx)
}

func (r *Repository) ListStories(ctx context.Context, includeUnpublished bool) ([]Story, error) {
	query := `
SELECT
	s.id,
	s.title,
	s.body,
	s.photo_id,
	ph.public_url,
	ph.caption,
	ph.alt_text,
	s.sort_order,
	s.is_published,
	s.created_at,
	s.updated_at
FROM cat_profile_stories s
LEFT JOIN cat_profile_photos ph ON ph.id = s.photo_id
`
	if !includeUnpublished {
		query += ` WHERE s.is_published = TRUE `
	}
	query += ` ORDER BY s.sort_order ASC, s.id ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Story
	for rows.Next() {
		story, err := scanStory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *story)
	}

	return out, rows.Err()
}

func (r *Repository) CreateStory(ctx context.Context, s Story) (*Story, error) {
	row := r.db.QueryRowContext(ctx, `
INSERT INTO cat_profile_stories (title, body, photo_id, sort_order, is_published)
VALUES ($1, $2, $3, $4, $5)
RETURNING id
`, s.Title, s.Body, s.PhotoID, s.SortOrder, s.IsPublished)

	var id int64
	if err := row.Scan(&id); err != nil {
		return nil, err
	}

	return r.GetStoryByID(ctx, id)
}

func (r *Repository) UpdateStory(ctx context.Context, id int64, s Story) (*Story, error) {
	res, err := r.db.ExecContext(ctx, `
UPDATE cat_profile_stories
SET
	title = $2,
	body = $3,
	photo_id = $4,
	sort_order = $5,
	is_published = $6,
	updated_at = NOW()
WHERE id = $1
`, id, s.Title, s.Body, s.PhotoID, s.SortOrder, s.IsPublished)
	if err != nil {
		return nil, err
	}

	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return nil, ErrNotFound
	}

	return r.GetStoryByID(ctx, id)
}

func (r *Repository) GetStoryByID(ctx context.Context, id int64) (*Story, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
	s.id,
	s.title,
	s.body,
	s.photo_id,
	ph.public_url,
	ph.caption,
	ph.alt_text,
	s.sort_order,
	s.is_published,
	s.created_at,
	s.updated_at
FROM cat_profile_stories s
LEFT JOIN cat_profile_photos ph ON ph.id = s.photo_id
WHERE s.id = $1
LIMIT 1
`, id)

	story, err := scanStory(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return story, nil
}

func (r *Repository) DeleteStory(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM cat_profile_stories WHERE id = $1`, id)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}

	return err
}

func (r *Repository) ListPhotos(ctx context.Context, includeUnpublished bool) ([]Photo, error) {
	query := `
SELECT id, filename, original_filename, content_type, file_path, public_url, caption, alt_text, sort_order, is_published, created_at, updated_at
FROM cat_profile_photos
`
	if !includeUnpublished {
		query += ` WHERE is_published = TRUE `
	}
	query += ` ORDER BY sort_order ASC, id ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Photo
	for rows.Next() {
		var p Photo
		if err := rows.Scan(
			&p.ID,
			&p.Filename,
			&p.OriginalFilename,
			&p.ContentType,
			&p.FilePath,
			&p.PublicURL,
			&p.Caption,
			&p.AltText,
			&p.SortOrder,
			&p.IsPublished,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	return out, rows.Err()
}

func (r *Repository) GetPhotoByFilename(ctx context.Context, filename string) (*Photo, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, filename, original_filename, content_type, file_path, public_url, caption, alt_text, sort_order, is_published, created_at, updated_at
FROM cat_profile_photos
WHERE filename = $1
LIMIT 1
`, filename)

	var p Photo
	err := row.Scan(
		&p.ID,
		&p.Filename,
		&p.OriginalFilename,
		&p.ContentType,
		&p.FilePath,
		&p.PublicURL,
		&p.Caption,
		&p.AltText,
		&p.SortOrder,
		&p.IsPublished,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &p, nil
}

func (r *Repository) CreatePhoto(ctx context.Context, p Photo) (*Photo, error) {
	row := r.db.QueryRowContext(ctx, `
INSERT INTO cat_profile_photos (
	filename, original_filename, content_type, file_path, public_url,
	caption, alt_text, sort_order, is_published
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
RETURNING id, filename, original_filename, content_type, file_path, public_url, caption, alt_text, sort_order, is_published, created_at, updated_at
`,
		p.Filename,
		p.OriginalFilename,
		p.ContentType,
		p.FilePath,
		p.PublicURL,
		p.Caption,
		p.AltText,
		p.SortOrder,
		p.IsPublished,
	)

	var out Photo
	if err := row.Scan(
		&out.ID,
		&out.Filename,
		&out.OriginalFilename,
		&out.ContentType,
		&out.FilePath,
		&out.PublicURL,
		&out.Caption,
		&out.AltText,
		&out.SortOrder,
		&out.IsPublished,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &out, nil
}

func (r *Repository) UpdatePhoto(ctx context.Context, id int64, p Photo) (*Photo, error) {
	row := r.db.QueryRowContext(ctx, `
UPDATE cat_profile_photos
SET
	caption = $2,
	alt_text = $3,
	sort_order = $4,
	is_published = $5,
	updated_at = NOW()
WHERE id = $1
RETURNING id, filename, original_filename, content_type, file_path, public_url, caption, alt_text, sort_order, is_published, created_at, updated_at
`, id, p.Caption, p.AltText, p.SortOrder, p.IsPublished)

	var out Photo
	err := row.Scan(
		&out.ID,
		&out.Filename,
		&out.OriginalFilename,
		&out.ContentType,
		&out.FilePath,
		&out.PublicURL,
		&out.Caption,
		&out.AltText,
		&out.SortOrder,
		&out.IsPublished,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &out, nil
}

func (r *Repository) DeletePhoto(ctx context.Context, id int64) (*Photo, error) {
	row := r.db.QueryRowContext(ctx, `
DELETE FROM cat_profile_photos
WHERE id = $1
RETURNING id, filename, original_filename, content_type, file_path, public_url, caption, alt_text, sort_order, is_published, created_at, updated_at
`, id)

	var p Photo
	err := row.Scan(
		&p.ID,
		&p.Filename,
		&p.OriginalFilename,
		&p.ContentType,
		&p.FilePath,
		&p.PublicURL,
		&p.Caption,
		&p.AltText,
		&p.SortOrder,
		&p.IsPublished,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &p, nil
}

type storyScanner interface {
	Scan(dest ...any) error
}

func scanStory(scanner storyScanner) (*Story, error) {
	var s Story

	var photoID sql.NullInt64
	var photoURL sql.NullString
	var photoCaption sql.NullString
	var photoAltText sql.NullString

	err := scanner.Scan(
		&s.ID,
		&s.Title,
		&s.Body,
		&photoID,
		&photoURL,
		&photoCaption,
		&photoAltText,
		&s.SortOrder,
		&s.IsPublished,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if photoID.Valid {
		s.PhotoID = &photoID.Int64
	}
	if photoURL.Valid {
		s.PhotoURL = &photoURL.String
	}
	if photoCaption.Valid {
		s.PhotoCaption = &photoCaption.String
	}
	if photoAltText.Valid {
		s.PhotoAltText = &photoAltText.String
	}

	return &s, nil
}
