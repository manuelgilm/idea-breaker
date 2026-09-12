package service

import (
	"context"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"

	"aibreak/internal/domain"
)

func newID() string { return ulid.Make().String() }

// duplicateTitleWarning is surfaced (not returned as an error) when a new
// idea's title matches an existing idea case-insensitively.
const duplicateTitleWarning = "an idea with a similar title already exists"

func (s *Service) CreateIdea(ctx context.Context, title, body string, tags []string) (domain.Idea, string, error) {
	if title == "" {
		return domain.Idea{}, "", fmt.Errorf("%w: title is required", ErrValidation)
	}
	if tags == nil {
		tags = []string{}
	}

	existing, err := s.store.FindIdeasByTitle(ctx, title)
	if err != nil {
		return domain.Idea{}, "", err
	}
	var warning string
	if len(existing) > 0 {
		warning = duplicateTitleWarning
	}

	now := time.Now().UTC()
	idea := domain.Idea{
		ID:      newID(),
		Title:   title,
		Body:    body,
		Tags:    tags,
		Created: now,
		Updated: now,
	}
	created, err := s.store.CreateIdea(ctx, idea)
	if err != nil {
		return domain.Idea{}, "", err
	}
	return created, warning, nil
}

func (s *Service) GetIdea(ctx context.Context, id string) (domain.Idea, error) {
	return s.store.GetIdea(ctx, id)
}

func (s *Service) ListIdeas(ctx context.Context, tag string) ([]domain.Idea, error) {
	return s.store.ListIdeas(ctx, tag)
}

func (s *Service) UpdateIdea(ctx context.Context, id string, patch domain.IdeaPatch) (domain.Idea, error) {
	idea, err := s.store.GetIdea(ctx, id)
	if err != nil {
		return domain.Idea{}, err
	}

	if patch.Title != nil {
		if *patch.Title == "" {
			return domain.Idea{}, fmt.Errorf("%w: title is required", ErrValidation)
		}
		idea.Title = *patch.Title
	}
	if patch.Body != nil {
		idea.Body = *patch.Body
	}
	if patch.Tags != nil {
		idea.Tags = *patch.Tags
	}
	idea.Updated = time.Now().UTC()

	return s.store.UpdateIdea(ctx, idea)
}

func (s *Service) DeleteIdea(ctx context.Context, id string) error {
	return s.store.DeleteIdea(ctx, id)
}
