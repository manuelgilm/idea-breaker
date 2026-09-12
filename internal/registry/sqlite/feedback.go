package sqlite

import (
	"context"
	"fmt"

	"aibreak/internal/domain"
	"aibreak/internal/registry"
)

func (s *Store) AddFeedback(ctx context.Context, f domain.Feedback) (domain.Feedback, error) {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO feedbacks (id, idea_id, author, score, rationale, aspect, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.IdeaID, f.Author, f.Score, f.Rationale, f.Aspect, formatTime(f.Created)); err != nil {
		return domain.Feedback{}, err
	}
	return f, nil
}

func (s *Store) ListFeedback(ctx context.Context, ideaID string) ([]domain.Feedback, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, idea_id, author, score, rationale, aspect, created_at FROM feedbacks WHERE idea_id = ? ORDER BY created_at ASC`, ideaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feedbacks := make([]domain.Feedback, 0)
	for rows.Next() {
		var f domain.Feedback
		var created string
		if err := rows.Scan(&f.ID, &f.IdeaID, &f.Author, &f.Score, &f.Rationale, &f.Aspect, &created); err != nil {
			return nil, err
		}
		if f.Created, err = parseTime(created); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, f)
	}
	return feedbacks, rows.Err()
}

func (s *Store) DeleteFeedback(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM feedbacks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: %s", registry.ErrNotFound, id)
	}
	return nil
}
