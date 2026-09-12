package service

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"aibreak/internal/domain"
)

var validKinds = map[string]bool{
	domain.KindArticle: true,
	domain.KindRepo:    true,
	domain.KindPaper:   true,
	domain.KindVideo:   true,
	domain.KindOther:   true,
}

func (s *Service) AddResource(ctx context.Context, ideaID, resourceURL, title, kind, note string) (domain.Resource, error) {
	if !validURL(resourceURL) {
		return domain.Resource{}, fmt.Errorf("%w: invalid URL", ErrValidation)
	}
	if kind == "" {
		kind = domain.KindOther
	}
	if !validKinds[kind] {
		return domain.Resource{}, fmt.Errorf("%w: invalid kind %q", ErrValidation, kind)
	}
	if _, err := s.store.GetIdea(ctx, ideaID); err != nil {
		return domain.Resource{}, err
	}

	r := domain.Resource{
		ID:      newID(),
		IdeaID:  ideaID,
		URL:     resourceURL,
		Title:   title,
		Kind:    kind,
		Note:    note,
		Created: time.Now().UTC(),
	}
	return s.store.AddResource(ctx, r)
}

func (s *Service) ListResources(ctx context.Context, ideaID string) ([]domain.Resource, error) {
	return s.store.ListResources(ctx, ideaID)
}

func (s *Service) DeleteResource(ctx context.Context, id string) error {
	return s.store.DeleteResource(ctx, id)
}

func validURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
