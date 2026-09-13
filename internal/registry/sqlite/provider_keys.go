package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"aibreak/internal/domain"
	"aibreak/internal/registry"
)

func (s *Store) AddProviderKey(ctx context.Context, k domain.APIKey) (domain.APIKey, error) {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO provider_keys (id, provider, label, hint, is_default, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		k.ID, k.Provider, k.Label, k.Hint, boolInt(k.IsDefault), formatTime(k.Created)); err != nil {
		return domain.APIKey{}, err
	}
	return k, nil
}

func (s *Store) GetProviderKey(ctx context.Context, id string) (domain.APIKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider, label, hint, is_default, created_at FROM provider_keys WHERE id = ?`, id)
	return scanProviderKey(row)
}

func (s *Store) ListProviderKeys(ctx context.Context, provider string) ([]domain.APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, provider, label, hint, is_default, created_at FROM provider_keys WHERE provider = ? ORDER BY created_at ASC, id ASC`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]domain.APIKey, 0)
	for rows.Next() {
		k, err := scanProviderKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) DeleteProviderKey(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM provider_keys WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: %s", registry.ErrNotFound, id)
	}
	return nil
}

// SetDefaultProviderKey clears any existing default for the key's provider and
// marks the given key default, atomically.
func (s *Store) SetDefaultProviderKey(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var provider string
	if err := tx.QueryRowContext(ctx, `SELECT provider FROM provider_keys WHERE id = ?`, id).Scan(&provider); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %s", registry.ErrNotFound, id)
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE provider_keys SET is_default = 0 WHERE provider = ?`, provider); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE provider_keys SET is_default = 1 WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetDefaultProviderKey(ctx context.Context, provider string) (domain.APIKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider, label, hint, is_default, created_at FROM provider_keys WHERE provider = ? AND is_default = 1 LIMIT 1`, provider)
	return scanProviderKey(row)
}

func scanProviderKey(row rowScanner) (domain.APIKey, error) {
	var k domain.APIKey
	var isDefault int
	var created string
	if err := row.Scan(&k.ID, &k.Provider, &k.Label, &k.Hint, &isDefault, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.APIKey{}, registry.ErrNotFound
		}
		return domain.APIKey{}, err
	}
	k.IsDefault = isDefault != 0
	var err error
	if k.Created, err = parseTime(created); err != nil {
		return domain.APIKey{}, err
	}
	return k, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
