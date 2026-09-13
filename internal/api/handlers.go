package api

import (
	"net/http"

	"aibreak/internal/domain"
)

type ideaRequest struct {
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

type ideaPatchRequest struct {
	Title *string   `json:"title"`
	Body  *string   `json:"body"`
	Tags  *[]string `json:"tags"`
}

type evaluateRequest struct {
	Personas []string `json:"personas"`
	Summary  bool     `json:"summary"`
}

type feedbackRequest struct {
	Author    string `json:"author"`
	Score     int    `json:"score"`
	Rationale string `json:"rationale"`
	Aspect    string `json:"aspect"`
}

type resourceRequest struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Note  string `json:"note"`
}

type personaRequest struct {
	Name         string  `json:"name"`
	SystemPrompt string  `json:"system_prompt"`
	Weight       float64 `json:"weight"`
}

type personaPatchRequest struct {
	Name         *string  `json:"name"`
	SystemPrompt *string  `json:"system_prompt"`
	Weight       *float64 `json:"weight"`
}

func (s *Server) createIdea(w http.ResponseWriter, r *http.Request) {
	var req ideaRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	idea, warning, err := s.svc.CreateIdea(r.Context(), req.Title, req.Body, req.Tags)
	if err != nil {
		writeError(w, err)
		return
	}
	if warning != "" {
		w.Header().Set("X-Warning", warning)
	}
	writeJSON(w, http.StatusCreated, idea)
}

func (s *Server) listIdeas(w http.ResponseWriter, r *http.Request) {
	ideas, err := s.svc.ListIdeas(r.Context(), r.URL.Query().Get("tag"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ideas)
}

func (s *Server) getIdea(w http.ResponseWriter, r *http.Request) {
	idea, err := s.svc.GetIdea(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, idea)
}

func (s *Server) updateIdea(w http.ResponseWriter, r *http.Request) {
	var req ideaPatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	idea, err := s.svc.UpdateIdea(r.Context(), r.PathValue("id"), domain.IdeaPatch{
		Title: req.Title,
		Body:  req.Body,
		Tags:  req.Tags,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, idea)
}

func (s *Server) deleteIdea(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteIdea(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) evaluate(w http.ResponseWriter, r *http.Request) {
	var req evaluateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	score, err := s.svc.Evaluate(r.Context(), r.PathValue("id"), req.Personas, req.Summary)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, score)
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.svc.ListRuns(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) addFeedback(w http.ResponseWriter, r *http.Request) {
	var req feedbackRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	f, err := s.svc.AddFeedback(r.Context(), r.PathValue("id"), req.Author, req.Score, req.Rationale, req.Aspect)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) listFeedback(w http.ResponseWriter, r *http.Request) {
	feedback, err := s.svc.ListFeedback(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, feedback)
}

func (s *Server) deleteFeedback(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteFeedback(r.Context(), r.PathValue("fid")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addResource(w http.ResponseWriter, r *http.Request) {
	var req resourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	res, err := s.svc.AddResource(r.Context(), r.PathValue("id"), req.URL, req.Title, req.Kind, req.Note)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) listResources(w http.ResponseWriter, r *http.Request) {
	resources, err := s.svc.ListResources(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resources)
}

func (s *Server) deleteResource(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteResource(r.Context(), r.PathValue("rid")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listPersonas(w http.ResponseWriter, r *http.Request) {
	personas, err := s.svc.ListPersonas(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, personas)
}

func (s *Server) createPersona(w http.ResponseWriter, r *http.Request) {
	var req personaRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	p, err := s.svc.CreatePersona(r.Context(), req.Name, req.SystemPrompt, req.Weight)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) updatePersona(w http.ResponseWriter, r *http.Request) {
	var req personaPatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	p, err := s.svc.UpdatePersona(r.Context(), r.PathValue("id"), domain.PersonaPatch{
		Name:         req.Name,
		SystemPrompt: req.SystemPrompt,
		Weight:       req.Weight,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deletePersona(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeletePersona(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
