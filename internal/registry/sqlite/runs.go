package sqlite

import (
	"context"
	"database/sql"

	"aibreak/internal/domain"
)

func (s *Store) SaveRun(ctx context.Context, score domain.FeasibilityScore, evals []domain.Evaluation) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Rollback is a no-op after a successful Commit (returns sql.ErrTxDone),
	// so its result is intentionally ignored here.
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO runs (run_id, idea_id, total, requested, responded, summary, verdict, spread, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		score.RunID, score.IdeaID, score.Total, score.Requested, score.Responded, score.Summary, score.Verdict, score.Spread, formatTime(score.Created)); err != nil {
		return err
	}

	for _, ev := range evals {
		var scoreVal any
		if ev.Status == domain.StatusSuccess {
			scoreVal = ev.Score
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO evaluations (run_id, idea_id, persona_id, persona_version, weight, status, score, rationale, error, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ev.RunID, ev.IdeaID, ev.PersonaID, ev.PersonaVersion, ev.Weight, ev.Status,
			scoreVal, ev.Rationale, ev.Error, formatTime(ev.Created)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListRuns(ctx context.Context, ideaID string) ([]domain.FeasibilityScore, error) {
	runRows, err := s.db.QueryContext(ctx,
		`SELECT run_id, idea_id, total, requested, responded, summary, verdict, spread, created_at FROM runs WHERE idea_id = ? ORDER BY run_id ASC`, ideaID)
	if err != nil {
		return nil, err
	}
	defer runRows.Close()

	runs := make([]domain.FeasibilityScore, 0)
	index := make(map[string]int)
	for runRows.Next() {
		var r domain.FeasibilityScore
		var created string
		if err := runRows.Scan(&r.RunID, &r.IdeaID, &r.Total, &r.Requested, &r.Responded, &r.Summary, &r.Verdict, &r.Spread, &created); err != nil {
			return nil, err
		}
		if r.Created, err = parseTime(created); err != nil {
			return nil, err
		}
		r.Breakdown = []domain.Evaluation{}
		index[r.RunID] = len(runs)
		runs = append(runs, r)
	}
	if err := runRows.Err(); err != nil {
		return nil, err
	}

	evalRows, err := s.db.QueryContext(ctx,
		`SELECT run_id, idea_id, persona_id, persona_version, weight, status, score, rationale, error, created_at FROM evaluations WHERE idea_id = ? ORDER BY run_id ASC, persona_id ASC`, ideaID)
	if err != nil {
		return nil, err
	}
	defer evalRows.Close()

	for evalRows.Next() {
		var ev domain.Evaluation
		var score sql.NullInt64
		var created string
		if err := evalRows.Scan(&ev.RunID, &ev.IdeaID, &ev.PersonaID, &ev.PersonaVersion, &ev.Weight, &ev.Status, &score, &ev.Rationale, &ev.Error, &created); err != nil {
			return nil, err
		}
		if score.Valid {
			ev.Score = int(score.Int64)
		}
		if ev.Created, err = parseTime(created); err != nil {
			return nil, err
		}
		if i, ok := index[ev.RunID]; ok {
			runs[i].Breakdown = append(runs[i].Breakdown, ev)
		}
	}
	if err := evalRows.Err(); err != nil {
		return nil, err
	}

	return runs, nil
}
