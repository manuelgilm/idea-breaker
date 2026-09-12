package sqlite

import (
	"time"
)

func (s *Store) seed() error {
	now := time.Now().UTC().Format(time.RFC3339)
	for _, p := range builtinPersonas {
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO personas (id, name, system_prompt, weight, version, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			p.ID, p.Name, p.SystemPrompt, p.Weight, p.Version, now,
		); err != nil {
			return err
		}
	}
	return nil
}
