package service

import (
	"context"
	"fmt"

	"aibreak/internal/domain"
	"aibreak/internal/engine"
)

// Evaluate evaluates an idea with the selected personas (all personas when
// personaIDs is empty), persists the run atomically, and returns the score.
// When summarize is true, an additional best-effort synthesis pass populates
// Summary and Verdict on the score.
func (s *Service) Evaluate(ctx context.Context, ideaID string, personaIDs []string, summarize bool) (domain.FeasibilityScore, error) {
	return s.EvaluateWithProgress(ctx, ideaID, personaIDs, summarize, nil)
}

// EvaluateWithProgress is like Evaluate but invokes onResult (if non-nil) as
// each persona's result completes.
func (s *Service) EvaluateWithProgress(ctx context.Context, ideaID string, personaIDs []string, summarize bool, onResult engine.ResultFunc) (domain.FeasibilityScore, error) {
	idea, err := s.store.GetIdea(ctx, ideaID)
	if err != nil {
		return domain.FeasibilityScore{}, err
	}

	var personas []domain.Persona
	if len(personaIDs) == 0 {
		personas, err = s.store.ListPersonas(ctx)
		if err != nil {
			return domain.FeasibilityScore{}, err
		}
	} else {
		seen := make(map[string]struct{}, len(personaIDs))
		for _, id := range personaIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			p, err := s.store.GetPersona(ctx, id)
			if err != nil {
				return domain.FeasibilityScore{}, fmt.Errorf("%w: unknown persona %q", ErrValidation, id)
			}
			personas = append(personas, p)
		}
	}

	score, err := s.evaluator.EvaluateWithProgress(ctx, idea, personas, summarize, onResult)
	if err != nil {
		return domain.FeasibilityScore{}, err
	}

	if err := s.store.SaveRun(ctx, score, score.Breakdown); err != nil {
		return domain.FeasibilityScore{}, err
	}
	return score, nil
}

// ListRuns returns the evaluation history for an idea.
func (s *Service) ListRuns(ctx context.Context, ideaID string) ([]domain.FeasibilityScore, error) {
	return s.store.ListRuns(ctx, ideaID)
}
