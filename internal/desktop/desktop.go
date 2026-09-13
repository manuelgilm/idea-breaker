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
	"fmt"
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

	// settingActiveProvider stores the persisted active provider.
	settingActiveProvider = "active_provider"
)

// builtinPersonaIDs are the persona slugs shipped with the tool (§2 of the
// product spec). They are seeded by the store and are not editable/deletable;
// the adapter marks them read-only in the UI.
var builtinPersonaIDs = map[string]bool{
	"skeptic":  true,
	"optimist": true,
	"engineer": true,
}

// ProviderRouter selects the active LLM provider and updates its API key/model
// at runtime. The concrete llm.Switchable satisfies it.
type ProviderRouter interface {
	SetActive(name string)
	Active() string
	SetKey(name, key string)
	SetModel(name, model string)
	Names() []string
	Model(name string) string
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
	svc          *service.Service
	router       ProviderRouter
	secrets      SecretStore
	fallbackKeys map[string]string
	ctx          context.Context
}

// Option configures an App.
type Option func(*App)

// WithSecretStore overrides the keyring-backed secret store (tests).
func WithSecretStore(s SecretStore) Option {
	return func(a *App) { a.secrets = s }
}

// WithFallbackKeys sets the per-provider config/env API keys used when no
// keyring default exists for a provider.
func WithFallbackKeys(m map[string]string) Option {
	return func(a *App) { a.fallbackKeys = m }
}

// New creates the bound application around an already-wired service. The router
// selects the active LLM provider and updates its API key at runtime; it may be
// nil in tests that do not exercise the provider settings.
func New(svc *service.Service, router ProviderRouter, opts ...Option) *App {
	a := &App{
		svc:          svc,
		router:       router,
		secrets:      keyringSecretStore{},
		fallbackKeys: map[string]string{},
		ctx:          context.Background(),
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

// ProviderInfo describes an LLM provider managed by the settings view.
type ProviderInfo struct {
	Name   string `json:"name"`
	Model  string `json:"model"`
	Active bool   `json:"active"`
	HasKey bool   `json:"hasKey"`
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
		score, err := a.latestScore(idea.ID)
		if err != nil {
			return nil, err
		}
		card.Score = score
		cards = append(cards, card)
	}
	return cards, nil
}

func (a *App) latestScore(ideaID string) (*float64, error) {
	runs, err := a.svc.ListRuns(a.ctx, ideaID)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	total := runs[len(runs)-1].Total
	return &total, nil
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

func (a *App) DeleteFeedback(id string) error {
	return a.svc.DeleteFeedback(a.ctx, id)
}

// ---- Provider settings ----

// GetProviders returns the supported providers with the active one flagged.
func (a *App) GetProviders() []ProviderInfo {
	active := a.router.Active()
	infos := make([]ProviderInfo, 0, len(a.router.Names()))
	for _, name := range a.router.Names() {
		infos = append(infos, ProviderInfo{
			Name:   name,
			Model:  a.router.Model(name),
			Active: name == active,
			HasKey: a.providerHasKey(name),
		})
	}
	return infos
}

// providerHasKey reports whether a provider has a usable key: a keyring default
// or a non-empty config/env fallback.
func (a *App) providerHasKey(name string) bool {
	if _, err := a.svc.GetDefaultProviderKey(a.ctx, name); err == nil {
		return true
	}
	return a.fallbackKeys[name] != ""
}

// SetActiveProvider switches the active provider, persists it, and applies its
// default key.
func (a *App) SetActiveProvider(name string) error {
	if !a.hasProvider(name) {
		return fmt.Errorf("unknown provider %q", name)
	}
	a.router.SetActive(name)
	if err := a.svc.SetSetting(a.ctx, settingActiveProvider, name); err != nil {
		return err
	}
	return a.applyDefault()
}

// SaveModel sets and persists a provider's model.
func (a *App) SaveModel(provider, model string) error {
	if !a.hasProvider(provider) {
		return fmt.Errorf("unknown provider %q", provider)
	}
	if model == "" {
		return fmt.Errorf("model must be non-empty")
	}
	a.router.SetModel(provider, model)
	return a.svc.SetSetting(a.ctx, modelSettingKey(provider), model)
}

// ApplySettings loads persisted settings (active provider and per-provider
// models) and applies them to the router, overriding the config/env defaults.
// It is called once at startup, before ApplyDefaultKey.
func (a *App) ApplySettings() error {
	if p, err := a.svc.GetSetting(a.ctx, settingActiveProvider); err != nil {
		return err
	} else if p != "" {
		a.router.SetActive(p)
	}
	for _, name := range a.router.Names() {
		if m, err := a.svc.GetSetting(a.ctx, modelSettingKey(name)); err != nil {
			return err
		} else if m != "" {
			a.router.SetModel(name, m)
		}
	}
	return nil
}

func (a *App) hasProvider(name string) bool {
	for _, n := range a.router.Names() {
		if n == name {
			return true
		}
	}
	return false
}

func modelSettingKey(provider string) string {
	return provider + ".model"
}

// ListAPIKeys returns a provider's registered keys' metadata (secrets stay in
// the keyring).
func (a *App) ListAPIKeys(provider string) ([]domain.APIKey, error) {
	return a.svc.ListProviderKeys(a.ctx, provider)
}

// AddAPIKey stores a new key's secret in the keyring and its metadata in the
// store. The first key for the provider becomes the default and is applied.
func (a *App) AddAPIKey(provider, label, key string) (domain.APIKey, error) {
	if key == "" {
		return domain.APIKey{}, errors.New("API key is required")
	}
	id := ulid.Make().String()
	account := keyAccount(provider, id)
	if err := a.secrets.Set(keyringService, account, key); err != nil {
		return domain.APIKey{}, err
	}
	k, err := a.svc.AddProviderKey(a.ctx, domain.APIKey{
		ID:       id,
		Provider: provider,
		Label:    label,
		Hint:     maskHint(key),
	})
	if err != nil {
		_ = a.secrets.Delete(keyringService, account)
		return domain.APIKey{}, err
	}
	if k.IsDefault && k.Provider == a.router.Active() {
		return k, a.applyDefault()
	}
	return k, nil
}

// DeleteAPIKey removes a key's secret and metadata. If it was the default,
// the service promotes another key (and the active default is re-applied).
func (a *App) DeleteAPIKey(id string) error {
	k, err := a.svc.GetProviderKey(a.ctx, id)
	if err != nil {
		return err
	}
	if err := a.svc.DeleteProviderKey(a.ctx, id); err != nil {
		return err
	}
	_ = a.secrets.Delete(keyringService, keyAccount(k.Provider, id))
	return a.applyDefault()
}

// SetDefaultAPIKey marks a key default and applies it to the running provider.
func (a *App) SetDefaultAPIKey(id string) error {
	if err := a.svc.SetDefaultProviderKey(a.ctx, id); err != nil {
		return err
	}
	return a.applyDefault()
}

// ApplyDefaultKey applies the active provider's default key (if any) to the
// running provider. It is called once at startup so the keyring default
// overrides the config key.
func (a *App) ApplyDefaultKey() error {
	return a.applyDefault()
}

func (a *App) applyDefault() error {
	active := a.router.Active()
	def, err := a.svc.GetDefaultProviderKey(a.ctx, active)
	if errors.Is(err, service.ErrNotFound) {
		a.router.SetKey(active, a.fallbackKeys[active])
		return nil
	}
	if err != nil {
		return err
	}
	secret, err := a.secrets.Get(keyringService, keyAccount(def.Provider, def.ID))
	if err != nil {
		return err
	}
	a.router.SetKey(active, secret)
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
