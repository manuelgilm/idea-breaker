package sqlite

import (
	"context"
	"fmt"

	"aibreak/internal/domain"
	"aibreak/internal/registry"
)

func (s *Store) AddResource(ctx context.Context, r domain.Resource) (domain.Resource, error) {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO resources (id, idea_id, url, title, kind, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.IdeaID, r.URL, r.Title, r.Kind, r.Note, formatTime(r.Created)); err != nil {
		return domain.Resource{}, err
	}
	return r, nil
}

func (s *Store) ListResources(ctx context.Context, ideaID string) ([]domain.Resource, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, idea_id, url, title, kind, note, created_at FROM resources WHERE idea_id = ? ORDER BY created_at ASC`, ideaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resources := make([]domain.Resource, 0)
	for rows.Next() {
		var r domain.Resource
		var created string
		if err := rows.Scan(&r.ID, &r.IdeaID, &r.URL, &r.Title, &r.Kind, &r.Note, &created); err != nil {
			return nil, err
		}
		if r.Created, err = parseTime(created); err != nil {
			return nil, err
		}
		resources = append(resources, r)
	}
	return resources, rows.Err()
}

func (s *Store) DeleteResource(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, id)
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
