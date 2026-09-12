package service

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"aibreak/internal/domain"
)

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

func (s *Service) CreatePersona(ctx context.Context, id, name, systemPrompt string, weight float64, version string) (domain.Persona, error) {
	if !slugRe.MatchString(id) {
		return domain.Persona{}, fmt.Errorf("%w: persona id must match [a-z0-9-]+", ErrValidation)
	}
	if name == "" {
		return domain.Persona{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if systemPrompt == "" {
		return domain.Persona{}, fmt.Errorf("%w: system_prompt is required", ErrValidation)
	}
	if weight < 0 {
		return domain.Persona{}, fmt.Errorf("%w: weight must be >= 0", ErrValidation)
	}
	if version == "" {
		version = "1.0.0"
	}

	p := domain.Persona{
		ID:           id,
		Name:         name,
		SystemPrompt: systemPrompt,
		Weight:       weight,
		Version:      version,
		Created:      time.Now().UTC(),
	}
	return s.store.CreatePersona(ctx, p)
}

func (s *Service) ListPersonas(ctx context.Context) ([]domain.Persona, error) {
	return s.store.ListPersonas(ctx)
}

// UpdatePersona merges the provided patch fields into the stored persona.
// Version is required on every edit and must differ from the current one, so
// every change bumps the version captured by evaluations. Built-in personas
// are rejected by the store with a conflict error.
func (s *Service) UpdatePersona(ctx context.Context, id string, patch domain.PersonaPatch) (domain.Persona, error) {
	p, err := s.store.GetPersona(ctx, id)
	if err != nil {
		return domain.Persona{}, err
	}

	if patch.Version == nil || *patch.Version == "" {
		return domain.Persona{}, fmt.Errorf("%w: version is required and must be non-empty", ErrValidation)
	}
	if *patch.Version == p.Version {
		return domain.Persona{}, fmt.Errorf("%w: version must differ from current version %q", ErrValidation, p.Version)
	}
	if patch.Name != nil {
		if *patch.Name == "" {
			return domain.Persona{}, fmt.Errorf("%w: name must be non-empty", ErrValidation)
		}
		p.Name = *patch.Name
	}
	if patch.SystemPrompt != nil {
		if *patch.SystemPrompt == "" {
			return domain.Persona{}, fmt.Errorf("%w: system_prompt must be non-empty", ErrValidation)
		}
		p.SystemPrompt = *patch.SystemPrompt
	}
	if patch.Weight != nil {
		if *patch.Weight < 0 {
			return domain.Persona{}, fmt.Errorf("%w: weight must be >= 0", ErrValidation)
		}
		p.Weight = *patch.Weight
	}
	p.Version = *patch.Version

	return s.store.UpdatePersona(ctx, p)
}

func (s *Service) DeletePersona(ctx context.Context, id string) error {
	return s.store.DeletePersona(ctx, id)
}
