package service

import (
	"context"
	"fmt"
	"time"

	"aibreak/internal/domain"
)

func (s *Service) AddFeedback(ctx context.Context, ideaID, author string, score int, rationale, aspect string) (domain.Feedback, error) {
	if author == "" {
		return domain.Feedback{}, fmt.Errorf("%w: author is required", ErrValidation)
	}
	if score < 0 || score > 5 {
		return domain.Feedback{}, fmt.Errorf("%w: score must be in [0,5]", ErrValidation)
	}
	if _, err := s.store.GetIdea(ctx, ideaID); err != nil {
		return domain.Feedback{}, err
	}

	f := domain.Feedback{
		ID:        newID(),
		IdeaID:    ideaID,
		Author:    author,
		Score:     score,
		Rationale: rationale,
		Aspect:    aspect,
		Created:   time.Now().UTC(),
	}
	return s.store.AddFeedback(ctx, f)
}

func (s *Service) ListFeedback(ctx context.Context, ideaID string) ([]domain.Feedback, error) {
	return s.store.ListFeedback(ctx, ideaID)
}

func (s *Service) DeleteFeedback(ctx context.Context, id string) error {
	return s.store.DeleteFeedback(ctx, id)
}
