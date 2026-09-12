// Package service is the application layer: it orchestrates the engine and the
// registry.Store, and owns validation, ID/timestamp generation, and run
// persistence.
package service

import (
	"errors"

	"aibreak/internal/engine"
	"aibreak/internal/registry"
)

var (
	// ErrValidation indicates invalid input.
	ErrValidation = errors.New("service: validation error")
	// ErrNotFound aliases the registry not-found error.
	ErrNotFound = registry.ErrNotFound
	// ErrConflict aliases the registry conflict error.
	ErrConflict = registry.ErrConflict
)

// Service orchestrates evaluation and persistence.
type Service struct {
	store     registry.Store
	evaluator *engine.Evaluator
}

// New creates a Service.
func New(store registry.Store, evaluator *engine.Evaluator) *Service {
	return &Service{store: store, evaluator: evaluator}
}
