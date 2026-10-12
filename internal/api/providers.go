package api

import (
	"fmt"
	"net/http"
	"time"

	"aibreak/internal/service"
)

// Setting keys shared with the desktop adapter: the persisted active provider
// and each provider's model.
const (
	settingActiveProvider = "active_provider"
)

func modelSettingKey(provider string) string {
	return provider + ".model"
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

// ProviderInfo describes a supported LLM provider.
type ProviderInfo struct {
	Name   string `json:"name"`
	Model  string `json:"model"`
	Active bool   `json:"active"`
	HasKey bool   `json:"has_key"`
}

type providerUpdateRequest struct {
	Model  *string `json:"model"`
	Active *bool   `json:"active"`
}

func (s *Server) listIdeaCards(w http.ResponseWriter, r *http.Request) {
	ideas, err := s.svc.ListIdeas(r.Context(), "")
	if err != nil {
		writeError(w, err)
		return
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
		runs, err := s.svc.ListRuns(r.Context(), idea.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		if len(runs) > 0 {
			total := runs[len(runs)-1].Total
			card.Score = &total
		}
		cards = append(cards, card)
	}
	writeJSON(w, http.StatusOK, cards)
}

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.providerInfos())
}

func (s *Server) providerInfos() []ProviderInfo {
	if s.router == nil {
		return []ProviderInfo{}
	}
	active := s.router.Active()
	names := s.router.Names()
	infos := make([]ProviderInfo, 0, len(names))
	for _, name := range names {
		infos = append(infos, ProviderInfo{
			Name:   name,
			Model:  s.router.Model(name),
			Active: name == active,
			HasKey: s.fallbackKeys[name] != "",
		})
	}
	return infos
}

func (s *Server) updateProvider(w http.ResponseWriter, r *http.Request) {
	if s.router == nil {
		writeError(w, fmt.Errorf("%w: providers are not configured", service.ErrNotFound))
		return
	}
	name := r.PathValue("name")
	if !s.hasProvider(name) {
		writeError(w, fmt.Errorf("%w: unknown provider %q", service.ErrNotFound, name))
		return
	}
	var req providerUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if req.Model != nil {
		if *req.Model == "" {
			writeError(w, fmt.Errorf("%w: model must be non-empty", service.ErrValidation))
			return
		}
		s.router.SetModel(name, *req.Model)
		if err := s.svc.SetSetting(r.Context(), modelSettingKey(name), *req.Model); err != nil {
			writeError(w, err)
			return
		}
	}
	if req.Active != nil && *req.Active {
		s.router.SetActive(name)
		if err := s.svc.SetSetting(r.Context(), settingActiveProvider, name); err != nil {
			writeError(w, err)
			return
		}
	}
	for _, info := range s.providerInfos() {
		if info.Name == name {
			writeJSON(w, http.StatusOK, info)
			return
		}
	}
	writeError(w, fmt.Errorf("%w: unknown provider %q", service.ErrNotFound, name))
}

func (s *Server) hasProvider(name string) bool {
	if s.router == nil {
		return false
	}
	for _, n := range s.router.Names() {
		if n == name {
			return true
		}
	}
	return false
}
