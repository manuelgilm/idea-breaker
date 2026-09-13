// Package desktop exposes the service layer to the Wails frontend.
//
// It is a thin adapter: every bound method delegates to the same
// *service.Service the CLI and HTTP API use, with identical error semantics.
// No new domain behavior lives here.
//
// Wails does not inject context.Context into bound methods (it treats it as a
// regular parameter), so the adapter holds a background context and passes it
// to the service internally; bound methods never expose context.Context.
package desktop

import (
	"context"
	"errors"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/zalando/go-keyring"

	"aibreak/internal/domain"
	"aibreak/internal/service"
)

const (
	// keyringService names the keyring service; each key's secret lives under
	// account "<provider>:<keyID>".
	keyringService = "aibreak"

	// providerName is the only provider the settings screen manages in v1.
	providerName = "openai"
)

// builtinPersonaIDs are the persona slugs shipped with the tool (§2 of the
// product spec). They are seeded by the store and are not editable/deletable;
// the adapter marks them read-only in the UI.
var builtinPersonaIDs = map[string]bool{
	"skeptic":  true,
	"optimist": true,
	"engineer": true,
}

// KeySetter lets a caller update the provider's API key in place. The concrete
// llm provider satisfies it.
type KeySetter interface {
	SetAPIKey(key string)
}

// SecretStore persists secrets (the API key). It is the seam that lets tests
// swap in an in-memory implementation.
type SecretStore interface {
	Get(service, account string) (string, error)
	Set(service, account, password string) error
	Delete(service, account string) error
}

// keyringSecretStore is the production SecretStore backed by the OS keyring.
type keyringSecretStore struct{}

func (keyringSecretStore) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}

func (keyringSecretStore) Set(service, account, password string) error {
	return keyring.Set(service, account, password)
}

func (keyringSecretStore) Delete(service, account string) error {
	return keyring.Delete(service, account)
}

// App is the Wails-bound application object.
type App struct {
	svc      *service.Service
	provider KeySetter
	secrets  SecretStore
	model    string
	ctx      context.Context
}

// Option configures an App.
type Option func(*App)

// WithSecretStore overrides the keyring-backed secret store (tests).
func WithSecretStore(s SecretStore) Option {
	return func(a *App) { a.secrets = s }
}

// WithModel sets the model name reported by the provider view.
func WithModel(m string) Option {
	return func(a *App) { a.model = m }
}

// New creates the bound application around an already-wired service. The
// provider is used to swap the API key at runtime; it may be nil in tests that
// do not exercise the provider settings.
func New(svc *service.Service, provider KeySetter, opts ...Option) *App {
	a := &App{
		svc:      svc,
		provider: provider,
		secrets:  keyringSecretStore{},
		model:    "gpt-4o-mini",
		ctx:      context.Background(),
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

// IdeaCard is an idea plus the total of its latest run (nil when unevaluated).
type IdeaCard struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Tags    []string  `json:"tags"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	Score   *float64  `json:"score"`
}

// PersonaView is a persona plus a read-only flag for built-ins.
type PersonaView struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Prompt  string    `json:"prompt"`
	Weight  float64   `json:"weight"`
	Version int       `json:"version"`
	Created time.Time `json:"created"`
	Builtin bool      `json:"builtin"`
}

// ProviderInfo describes the LLM provider managed by the settings view.
type ProviderInfo struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// ---- Ideas ----

func (a *App) ListIdeas(tag string) ([]domain.Idea, error) {
	return a.svc.ListIdeas(a.ctx, tag)
}

// ListIdeaCards returns ideas with their latest run's total (for the card dot).
func (a *App) ListIdeaCards() ([]IdeaCard, error) {
	ideas, err := a.svc.ListIdeas(a.ctx, "")
	if err != nil {
		return nil, err
	}
	cards := make([]IdeaCard, 0, len(ideas))
	for _, idea := range ideas {
		card := IdeaCard{
			ID:      idea.ID,
			Title:   idea.Title,
			Body:    idea.Body,
			Tags:    idea.Tags,
			Created: idea.Created,
			Updated: idea.Updated,
		}
		if score := a.latestScore(idea.ID); score != nil {
			card.Score = score
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func (a *App) latestScore(ideaID string) *float64 {
	runs, err := a.svc.ListRuns(a.ctx, ideaID)
	if err != nil || len(runs) == 0 {
		return nil
	}
	total := runs[len(runs)-1].Total
	return &total
}

func (a *App) GetIdea(id string) (domain.Idea, error) {
	return a.svc.GetIdea(a.ctx, id)
}

func (a *App) CreateIdea(title, body string, tags []string) (domain.Idea, error) {
	idea, _, err := a.svc.CreateIdea(a.ctx, title, body, tags)
	return idea, err
}

func (a *App) UpdateIdea(id, title, body string, tags []string) (domain.Idea, error) {
	return a.svc.UpdateIdea(a.ctx, id, domain.IdeaPatch{Title: &title, Body: &body, Tags: &tags})
}

func (a *App) DeleteIdea(id string) error {
	return a.svc.DeleteIdea(a.ctx, id)
}

// ---- Evaluation ----

func (a *App) Evaluate(ideaID string, personaIDs []string, summarize bool) (domain.FeasibilityScore, error) {
	return a.svc.Evaluate(a.ctx, ideaID, personaIDs, summarize)
}

func (a *App) ListRuns(ideaID string) ([]domain.FeasibilityScore, error) {
	return a.svc.ListRuns(a.ctx, ideaID)
}

// ---- Personas ----

func (a *App) ListPersonas() ([]PersonaView, error) {
	personas, err := a.svc.ListPersonas(a.ctx)
	if err != nil {
		return nil, err
	}
	views := make([]PersonaView, 0, len(personas))
	for _, p := range personas {
		views = append(views, personaView(p))
	}
	return views, nil
}

func (a *App) CreatePersona(name, prompt string, weight float64) (PersonaView, error) {
	p, err := a.svc.CreatePersona(a.ctx, name, prompt, weight)
	if err != nil {
		return PersonaView{}, err
	}
	return personaView(p), nil
}

// UpdatePersona edits a custom persona in place and bumps its version.
func (a *App) UpdatePersona(id, name, prompt string, weight float64) (PersonaView, error) {
	p, err := a.svc.UpdatePersona(a.ctx, id, domain.PersonaPatch{
		Name:         &name,
		SystemPrompt: &prompt,
		Weight:       &weight,
	})
	if err != nil {
		return PersonaView{}, err
	}
	return personaView(p), nil
}

func (a *App) DeletePersona(id string) error {
	return a.svc.DeletePersona(a.ctx, id)
}

func personaView(p domain.Persona) PersonaView {
	return PersonaView{
		ID:      p.ID,
		Name:    p.Name,
		Prompt:  p.SystemPrompt,
		Weight:  p.Weight,
		Version: p.Version,
		Created: p.Created,
		Builtin: builtinPersonaIDs[p.ID],
	}
}

// ---- Resources ----

func (a *App) ListResources(ideaID string) ([]domain.Resource, error) {
	return a.svc.ListResources(a.ctx, ideaID)
}

func (a *App) AddResource(ideaID, resourceURL, title, kind, note string) (domain.Resource, error) {
	return a.svc.AddResource(a.ctx, ideaID, resourceURL, title, kind, note)
}

func (a *App) DeleteResource(id string) error {
	return a.svc.DeleteResource(a.ctx, id)
}

// ---- Feedback ----

func (a *App) ListFeedback(ideaID string) ([]domain.Feedback, error) {
	return a.svc.ListFeedback(a.ctx, ideaID)
}

func (a *App) AddFeedback(ideaID, author string, score int, rationale, aspect string) (domain.Feedback, error) {
	return a.svc.AddFeedback(a.ctx, ideaID, author, score, rationale, aspect)
}

// ---- Provider settings ----

func (a *App) GetProviderInfo() (ProviderInfo, error) {
	return ProviderInfo{Provider: providerName, Model: a.model}, nil
}

// ListAPIKeys returns the registered keys' metadata (secrets stay in the
// keyring).
func (a *App) ListAPIKeys() ([]domain.APIKey, error) {
	return a.svc.ListProviderKeys(a.ctx, providerName)
}

// AddAPIKey stores a new key's secret in the keyring and its metadata in the
// store. The first key for the provider becomes the default and is applied.
func (a *App) AddAPIKey(label, key string) (domain.APIKey, error) {
	if key == "" {
		return domain.APIKey{}, errors.New("API key is required")
	}
	id := ulid.Make().String()
	account := keyAccount(providerName, id)
	if err := a.secrets.Set(keyringService, account, key); err != nil {
		return domain.APIKey{}, err
	}
	k, err := a.svc.AddProviderKey(a.ctx, domain.APIKey{
		ID:       id,
		Provider: providerName,
		Label:    label,
		Hint:     maskHint(key),
	})
	if err != nil {
		return domain.APIKey{}, err
	}
	if k.IsDefault {
		a.applyDefault()
	}
	return k, nil
}

// DeleteAPIKey removes a key's secret and metadata. If it was the default,
// the service promotes another key (and it is re-applied).
func (a *App) DeleteAPIKey(id string) error {
	if err := a.svc.DeleteProviderKey(a.ctx, id); err != nil {
		return err
	}
	_ = a.secrets.Delete(keyringService, keyAccount(providerName, id))
	a.applyDefault()
	return nil
}

// SetDefaultAPIKey marks a key default and applies it to the running provider.
func (a *App) SetDefaultAPIKey(id string) error {
	if err := a.svc.SetDefaultProviderKey(a.ctx, id); err != nil {
		return err
	}
	a.applyDefault()
	return nil
}

// ApplyDefaultKey applies the default key (if any) to the running provider. It
// is called once at startup so the keyring default overrides the config key.
func (a *App) ApplyDefaultKey() error {
	return a.applyDefault()
}

func (a *App) applyDefault() error {
	def, err := a.svc.GetDefaultProviderKey(a.ctx, providerName)
	if errors.Is(err, service.ErrNotFound) {
		if a.provider != nil {
			a.provider.SetAPIKey("")
		}
		return nil
	}
	if err != nil {
		return err
	}
	secret, err := a.secrets.Get(keyringService, keyAccount(def.Provider, def.ID))
	if err != nil {
		return err
	}
	if a.provider != nil {
		a.provider.SetAPIKey(secret)
	}
	return nil
}

func keyAccount(provider, id string) string {
	return provider + ":" + id
}

// maskHint returns a non-reversible suffix for display (e.g. "••••wxyz9").
func maskHint(key string) string {
	const dot = "••••"
	if len(key) <= 4 {
		return dot
	}
	return dot + key[len(key)-4:]
}
