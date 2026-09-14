package llm

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Switchable routes chat completions to one of several named providers and
// keeps a single active provider at a time. It is used by the desktop app to
// let the user switch providers (and models) at runtime without rebuilding the
// engine.
type Switchable struct {
	mu        sync.RWMutex
	active    string
	providers map[string]KeyedProvider
	models    map[string]string
	order     []string
}

// NewSwitchable builds a router. active must be a key in providers.
func NewSwitchable(providers map[string]KeyedProvider, models map[string]string, active string) *Switchable {
	s := &Switchable{
		providers: providers,
		models:    models,
		order:     make([]string, 0, len(providers)),
	}
	for name := range providers {
		s.order = append(s.order, name)
	}
	sort.Strings(s.order)
	if _, ok := providers[active]; ok {
		s.active = active
	} else if len(s.order) > 0 {
		s.active = s.order[0]
	}
	return s
}

// Complete routes to the active provider, overriding the request model with
// the active provider's configured model.
func (s *Switchable) Complete(ctx context.Context, req Request) (Response, error) {
	s.mu.RLock()
	name := s.active
	model := s.models[name]
	p := s.providers[name]
	s.mu.RUnlock()

	if p == nil {
		return Response{}, fmt.Errorf("%w: no active provider %q", ErrProvider, name)
	}
	if model != "" {
		req.Model = model
	}
	return p.Complete(ctx, req)
}

// SetActive switches the active provider. It is a no-op for unknown names.
func (s *Switchable) SetActive(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.providers[name]; ok {
		s.active = name
	}
}

// Active returns the currently active provider name.
func (s *Switchable) Active() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// SetKey updates the API key of a named provider.
func (s *Switchable) SetKey(name, key string) {
	s.mu.RLock()
	p := s.providers[name]
	s.mu.RUnlock()
	if p != nil {
		p.SetAPIKey(key)
	}
}

// SetModel updates the model of a named provider. It is a no-op for unknown
// names.
func (s *Switchable) SetModel(name, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.providers[name]; ok {
		s.models[name] = model
	}
}

// Names returns the provider names in sorted order.
func (s *Switchable) Names() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.order...)
}

// Model returns the configured model for a provider name.
func (s *Switchable) Model(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.models[name]
}
