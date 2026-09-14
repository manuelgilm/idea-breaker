package service

import (
	"context"
	"fmt"
	"time"

	"aibreak/internal/domain"
)

// AddProviderKey persists provider-key metadata. The first key for a provider
// becomes its default. The caller owns the secret (keyring) and supplies a
// generated ID and a masked hint.
func (s *Service) AddProviderKey(ctx context.Context, k domain.APIKey) (domain.APIKey, error) {
	if k.ID == "" || k.Provider == "" || k.Hint == "" {
		return domain.APIKey{}, fmt.Errorf("%w: provider key requires id, provider, and hint", ErrValidation)
	}
	if k.Created.IsZero() {
		k.Created = time.Now().UTC()
	}
	existing, err := s.store.ListProviderKeys(ctx, k.Provider)
	if err != nil {
		return domain.APIKey{}, err
	}
	if len(existing) == 0 {
		k.IsDefault = true
	}
	return s.store.AddProviderKey(ctx, k)
}

func (s *Service) ListProviderKeys(ctx context.Context, provider string) ([]domain.APIKey, error) {
	return s.store.ListProviderKeys(ctx, provider)
}

func (s *Service) GetProviderKey(ctx context.Context, id string) (domain.APIKey, error) {
	return s.store.GetProviderKey(ctx, id)
}

// DeleteProviderKey removes a key's metadata. If the deleted key was the
// default, the most recently created remaining key is promoted to default.
func (s *Service) DeleteProviderKey(ctx context.Context, id string) error {
	k, err := s.store.GetProviderKey(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteProviderKey(ctx, id); err != nil {
		return err
	}
	if !k.IsDefault {
		return nil
	}
	remaining, err := s.store.ListProviderKeys(ctx, k.Provider)
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		return nil
	}
	return s.store.SetDefaultProviderKey(ctx, remaining[len(remaining)-1].ID)
}

func (s *Service) SetDefaultProviderKey(ctx context.Context, id string) error {
	return s.store.SetDefaultProviderKey(ctx, id)
}

func (s *Service) GetDefaultProviderKey(ctx context.Context, provider string) (domain.APIKey, error) {
	return s.store.GetDefaultProviderKey(ctx, provider)
}
