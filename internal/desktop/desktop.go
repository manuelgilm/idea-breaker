// Package desktop exposes the service layer to the Wails frontend.
//
// It is a thin adapter: every bound method delegates to the same
// *service.Service the CLI and HTTP API use, with identical signatures and
// error semantics. No new domain behavior lives here. Wails binds the
// exported methods of App; the leading context.Context is supplied by the
// Wails runtime.
package desktop

import (
	"context"

	"aibreak/internal/domain"
	"aibreak/internal/service"
)

// App is the Wails-bound application object.
type App struct {
	svc *service.Service
}

// New creates the bound application around an already-wired service.
func New(svc *service.Service) *App {
	return &App{svc: svc}
}

// ListIdeas returns all ideas, optionally filtered by tag.
func (a *App) ListIdeas(ctx context.Context, tag string) ([]domain.Idea, error) {
	return a.svc.ListIdeas(ctx, tag)
}

// GetIdea returns a single idea by id.
func (a *App) GetIdea(ctx context.Context, id string) (domain.Idea, error) {
	return a.svc.GetIdea(ctx, id)
}

// Evaluate runs persona evaluation on an idea. An empty personaIDs selects
// all personas; summarize requests the second-stage synthesis pass.
func (a *App) Evaluate(ctx context.Context, ideaID string, personaIDs []string, summarize bool) (domain.FeasibilityScore, error) {
	return a.svc.Evaluate(ctx, ideaID, personaIDs, summarize)
}

// ListRuns returns the evaluation history for an idea.
func (a *App) ListRuns(ctx context.Context, ideaID string) ([]domain.FeasibilityScore, error) {
	return a.svc.ListRuns(ctx, ideaID)
}

// ListFeedback returns the human feedback for an idea.
func (a *App) ListFeedback(ctx context.Context, ideaID string) ([]domain.Feedback, error) {
	return a.svc.ListFeedback(ctx, ideaID)
}

// AddFeedback records human feedback for an idea.
func (a *App) AddFeedback(ctx context.Context, ideaID, author string, score int, rationale, aspect string) (domain.Feedback, error) {
	return a.svc.AddFeedback(ctx, ideaID, author, score, rationale, aspect)
}
