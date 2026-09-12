package service

import (
	"context"
	"fmt"

	"aibreak/internal/domain"
)

// Evaluate evaluates an idea with the selected personas (all personas when
// personaIDs is empty), persists the run atomically, and returns the score.
// When summarize is true, an additional best-effort synthesis pass populates
// Summary and Verdict on the score.
func (s *Service) Evaluate(ctx context.Context, ideaID string, personaIDs []string, summarize bool) (domain.FeasibilityScore, error) {
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
		for _, id := range personaIDs {
			p, err := s.store.GetPersona(ctx, id)
			if err != nil {
				return domain.FeasibilityScore{}, fmt.Errorf("%w: unknown persona %q", ErrValidation, id)
			}
			personas = append(personas, p)
		}
	}

	score, err := s.evaluator.Evaluate(ctx, idea, personas, summarize)
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
