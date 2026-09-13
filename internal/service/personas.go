package service

import (
	"context"
	"fmt"
	"time"

	"aibreak/internal/domain"
)

func (s *Service) CreatePersona(ctx context.Context, name, systemPrompt string, weight float64) (domain.Persona, error) {
	if name == "" {
		return domain.Persona{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if systemPrompt == "" {
		return domain.Persona{}, fmt.Errorf("%w: system_prompt is required", ErrValidation)
	}
	if weight < 0 {
		return domain.Persona{}, fmt.Errorf("%w: weight must be >= 0", ErrValidation)
	}
	if weight == 0 {
		weight = 1.0
	}

	p := domain.Persona{
		ID:           newID(),
		Name:         name,
		SystemPrompt: systemPrompt,
		Weight:       weight,
		Version:      1,
		Created:      time.Now().UTC(),
	}
	return s.store.CreatePersona(ctx, p)
}

func (s *Service) ListPersonas(ctx context.Context) ([]domain.Persona, error) {
	return s.store.ListPersonas(ctx)
}

// UpdatePersona merges the provided patch fields into the stored persona and
// bumps its Version by one, so every change is captured by evaluations.
// Built-in personas are rejected by the store with a conflict error.
func (s *Service) UpdatePersona(ctx context.Context, id string, patch domain.PersonaPatch) (domain.Persona, error) {
	p, err := s.store.GetPersona(ctx, id)
	if err != nil {
		return domain.Persona{}, err
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
	p.Version++

	return s.store.UpdatePersona(ctx, p)
}

func (s *Service) DeletePersona(ctx context.Context, id string) error {
	return s.store.DeletePersona(ctx, id)
}
