package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"aibreak/internal/domain"
	"aibreak/internal/registry"
)

func (s *Store) CreatePersona(ctx context.Context, p domain.Persona) (domain.Persona, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM personas WHERE id = ?`, p.ID).Scan(&exists); err != nil {
		return domain.Persona{}, err
	}
	if exists > 0 {
		return domain.Persona{}, fmt.Errorf("%w: %s", registry.ErrConflict, p.ID)
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO personas (id, name, system_prompt, weight, version, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.SystemPrompt, p.Weight, p.Version, formatTime(p.Created)); err != nil {
		return domain.Persona{}, err
	}
	return p, nil
}

func (s *Store) GetPersona(ctx context.Context, id string) (domain.Persona, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, system_prompt, weight, version, created_at FROM personas WHERE id = ?`, id)
	return scanPersona(row)
}

func (s *Store) ListPersonas(ctx context.Context) ([]domain.Persona, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, system_prompt, weight, version, created_at FROM personas ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	personas := make([]domain.Persona, 0)
	for rows.Next() {
		p, err := scanPersona(rows)
		if err != nil {
			return nil, err
		}
		personas = append(personas, p)
	}
	return personas, rows.Err()
}

func (s *Store) UpdatePersona(ctx context.Context, p domain.Persona) (domain.Persona, error) {
	if builtinIDs[p.ID] {
		return domain.Persona{}, fmt.Errorf("%w: built-in persona %s is not editable", registry.ErrConflict, p.ID)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE personas SET name = ?, system_prompt = ?, weight = ?, version = ? WHERE id = ?`,
		p.Name, p.SystemPrompt, p.Weight, p.Version, p.ID)
	if err != nil {
		return domain.Persona{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.Persona{}, err
	}
	if n == 0 {
		return domain.Persona{}, fmt.Errorf("%w: %s", registry.ErrNotFound, p.ID)
	}
	return p, nil
}

func (s *Store) DeletePersona(ctx context.Context, id string) error {
	if builtinIDs[id] {
		return fmt.Errorf("%w: built-in persona %s is not deletable", registry.ErrConflict, id)
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM personas WHERE id = ?`, id)
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

func scanPersona(row rowScanner) (domain.Persona, error) {
	var p domain.Persona
	var created string
	if err := row.Scan(&p.ID, &p.Name, &p.SystemPrompt, &p.Weight, &p.Version, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Persona{}, registry.ErrNotFound
		}
		return domain.Persona{}, err
	}
	var err error
	if p.Created, err = parseTime(created); err != nil {
		return domain.Persona{}, err
	}
	return p, nil
}
