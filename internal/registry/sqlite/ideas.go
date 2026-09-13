package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"aibreak/internal/domain"
	"aibreak/internal/registry"
)

func (s *Store) CreateIdea(ctx context.Context, idea domain.Idea) (domain.Idea, error) {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO ideas (id, title, body, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		idea.ID, idea.Title, idea.Body, marshalTags(idea.Tags),
		formatTime(idea.Created), formatTime(idea.Updated)); err != nil {
		return domain.Idea{}, err
	}
	return idea, nil
}

func (s *Store) GetIdea(ctx context.Context, id string) (domain.Idea, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, body, tags, created_at, updated_at FROM ideas WHERE id = ?`, id)
	return scanIdea(row)
}

func (s *Store) ListIdeas(ctx context.Context, tag string) ([]domain.Idea, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, body, tags, created_at, updated_at FROM ideas ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ideas := make([]domain.Idea, 0)
	for rows.Next() {
		idea, err := scanIdea(rows)
		if err != nil {
			return nil, err
		}
		if tag != "" && !containsTag(idea.Tags, tag) {
			continue
		}
		ideas = append(ideas, idea)
	}
	return ideas, rows.Err()
}

// FindIdeasByTitle returns ideas whose title matches case-insensitively
// (compared on trimmed values).
func (s *Store) FindIdeasByTitle(ctx context.Context, title string) ([]domain.Idea, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, body, tags, created_at, updated_at FROM ideas WHERE LOWER(TRIM(title)) = LOWER(TRIM(?)) ORDER BY created_at DESC, id DESC`, title)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ideas := make([]domain.Idea, 0)
	for rows.Next() {
		idea, err := scanIdea(rows)
		if err != nil {
			return nil, err
		}
		ideas = append(ideas, idea)
	}
	return ideas, rows.Err()
}

func (s *Store) UpdateIdea(ctx context.Context, idea domain.Idea) (domain.Idea, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE ideas SET title = ?, body = ?, tags = ?, updated_at = ? WHERE id = ?`,
		idea.Title, idea.Body, marshalTags(idea.Tags), formatTime(idea.Updated), idea.ID)
	if err != nil {
		return domain.Idea{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Idea{}, err
	}
	if n == 0 {
		return domain.Idea{}, fmt.Errorf("%w: %s", registry.ErrNotFound, idea.ID)
	}
	return idea, nil
}

func (s *Store) DeleteIdea(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ideas WHERE id = ?`, id)
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanIdea(row rowScanner) (domain.Idea, error) {
	var idea domain.Idea
	var tags, created, updated string
	if err := row.Scan(&idea.ID, &idea.Title, &idea.Body, &tags, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Idea{}, registry.ErrNotFound
		}
		return domain.Idea{}, err
	}
	var err error
	if idea.Tags, err = unmarshalTags(tags); err != nil {
		return domain.Idea{}, err
	}
	if idea.Created, err = parseTime(created); err != nil {
		return domain.Idea{}, err
	}
	if idea.Updated, err = parseTime(updated); err != nil {
		return domain.Idea{}, err
	}
	return idea, nil
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
