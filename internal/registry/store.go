// Package registry defines the persistence contract (Store) for the engine and
// service. Implementations persist domain objects verbatim; ID and timestamp
// generation is the caller's responsibility.
package registry

import (
	"context"
	"errors"

	"aibreak/internal/domain"
)

var (
	// ErrNotFound indicates a requested entity does not exist.
	ErrNotFound = errors.New("registry: not found")
	// ErrConflict indicates a uniqueness violation (e.g. duplicate persona id).
	ErrConflict = errors.New("registry: conflict")
)

// Store persists ideas, personas, evaluation runs, feedback, and resources.
type Store interface {
	CreateIdea(ctx context.Context, idea domain.Idea) (domain.Idea, error)
	GetIdea(ctx context.Context, id string) (domain.Idea, error)
	ListIdeas(ctx context.Context, tag string) ([]domain.Idea, error)
	FindIdeasByTitle(ctx context.Context, title string) ([]domain.Idea, error)
	UpdateIdea(ctx context.Context, idea domain.Idea) (domain.Idea, error)
	DeleteIdea(ctx context.Context, id string) error

	CreatePersona(ctx context.Context, p domain.Persona) (domain.Persona, error)
	GetPersona(ctx context.Context, id string) (domain.Persona, error)
	ListPersonas(ctx context.Context) ([]domain.Persona, error)
	UpdatePersona(ctx context.Context, p domain.Persona) (domain.Persona, error)
	DeletePersona(ctx context.Context, id string) error

	SaveRun(ctx context.Context, score domain.FeasibilityScore, evals []domain.Evaluation) error
	ListRuns(ctx context.Context, ideaID string) ([]domain.FeasibilityScore, error)

	AddFeedback(ctx context.Context, f domain.Feedback) (domain.Feedback, error)
	ListFeedback(ctx context.Context, ideaID string) ([]domain.Feedback, error)
	DeleteFeedback(ctx context.Context, id string) error

	AddResource(ctx context.Context, r domain.Resource) (domain.Resource, error)
	ListResources(ctx context.Context, ideaID string) ([]domain.Resource, error)
	DeleteResource(ctx context.Context, id string) error

	AddProviderKey(ctx context.Context, k domain.APIKey) (domain.APIKey, error)
	GetProviderKey(ctx context.Context, id string) (domain.APIKey, error)
	ListProviderKeys(ctx context.Context, provider string) ([]domain.APIKey, error)
	DeleteProviderKey(ctx context.Context, id string) error
	SetDefaultProviderKey(ctx context.Context, id string) error
	GetDefaultProviderKey(ctx context.Context, provider string) (domain.APIKey, error)

	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}
