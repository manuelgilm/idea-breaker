package service

import "context"

func (s *Service) GetSetting(ctx context.Context, key string) (string, error) {
	return s.store.GetSetting(ctx, key)
}

func (s *Service) SetSetting(ctx context.Context, key, value string) error {
	return s.store.SetSetting(ctx, key, value)
}
