package missingquestions

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type MissingQuestion struct {
	ID             int64          `json:"id"`
	SessionID      string         `json:"session_id,omitempty"`
	Mode           string         `json:"mode,omitempty"`
	Question       string         `json:"question"`
	RewrittenQuery string         `json:"rewritten_query,omitempty"`
	Reason         string         `json:"reason"`
	Filters        map[string]any `json:"filters,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, q MissingQuestion) error {
	filterBytes, err := json.Marshal(q.Filters)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO missing_questions (
	session_id, mode, question, rewritten_query, reason, filters
)
VALUES ($1, $2, $3, $4, $5, $6::jsonb)
`,
		q.SessionID,
		q.Mode,
		q.Question,
		q.RewrittenQuery,
		q.Reason,
		string(filterBytes),
	)

	return err
}

func (r *Repository) List(ctx context.Context, limit int) ([]MissingQuestion, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT id, session_id, mode, question, rewritten_query, reason, filters, created_at
FROM missing_questions
ORDER BY created_at DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MissingQuestion

	for rows.Next() {
		var q MissingQuestion
		var filterBytes []byte

		if err := rows.Scan(
			&q.ID,
			&q.SessionID,
			&q.Mode,
			&q.Question,
			&q.RewrittenQuery,
			&q.Reason,
			&filterBytes,
			&q.CreatedAt,
		); err != nil {
			return nil, err
		}

		if len(filterBytes) > 0 {
			_ = json.Unmarshal(filterBytes, &q.Filters)
		}

		out = append(out, q)
	}

	return out, rows.Err()
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `
DELETE FROM missing_questions
WHERE id = $1
`, id)
	return err
}

func (r *Repository) Clear(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
DELETE FROM missing_questions
`)
	return err
}

func (r *Repository) LogMissingQuestion(
	ctx context.Context,
	sessionID string,
	mode string,
	question string,
	rewrittenQuery string,
	reason string,
	filters map[string]any,
) error {
	return r.Create(ctx, MissingQuestion{
		SessionID:      sessionID,
		Mode:           mode,
		Question:       question,
		RewrittenQuery: rewrittenQuery,
		Reason:         reason,
		Filters:        filters,
	})
}
