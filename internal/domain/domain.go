package domain

import "time"

// Evaluation statuses.
const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// Resource kinds.
const (
	KindArticle = "article"
	KindRepo    = "repo"
	KindPaper   = "paper"
	KindVideo   = "video"
	KindOther   = "other"
)

// Idea is a candidate product idea.
type Idea struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Tags    []string  `json:"tags"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// Persona defines an evaluation perspective.
type Persona struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SystemPrompt string    `json:"system_prompt"`
	Weight       float64   `json:"weight"`
	Version      string    `json:"version"`
	Created      time.Time `json:"created"`
}

// Evaluation is the result of one persona evaluating one idea.
type Evaluation struct {
	RunID          string    `json:"run_id"`
	IdeaID         string    `json:"idea_id"`
	PersonaID      string    `json:"persona_id"`
	PersonaVersion string    `json:"persona_version"`
	Weight         float64   `json:"weight"`
	Status         string    `json:"status"`
	Score          int       `json:"score"`
	Rationale      string    `json:"rationale"`
	Error          string    `json:"error,omitempty"`
	Created        time.Time `json:"created"`
}

// Verdict labels for the optional synthesis stage.
const (
	VerdictProceed      = "proceed"
	VerdictPromising    = "promising"
	VerdictRisky        = "risky"
	VerdictPass         = "pass"
	VerdictInconclusive = "inconclusive"
)

// FeasibilityScore is the aggregate of one evaluation run.
type FeasibilityScore struct {
	RunID     string       `json:"run_id"`
	IdeaID    string       `json:"idea_id"`
	Total     float64      `json:"total"`
	Breakdown []Evaluation `json:"breakdown"`
	Requested int          `json:"requested"`
	Responded int          `json:"responded"`
	Spread    float64      `json:"spread"`
	Summary   string       `json:"summary,omitempty"`
	Verdict   string       `json:"verdict,omitempty"`
	Created   time.Time    `json:"created"`
}

// Feedback is a human review of an idea.
type Feedback struct {
	ID        string    `json:"id"`
	IdeaID    string    `json:"idea_id"`
	Author    string    `json:"author"`
	Score     int       `json:"score"`
	Rationale string    `json:"rationale"`
	Aspect    string    `json:"aspect,omitempty"`
	Created   time.Time `json:"created"`
}

// Resource is an external research item attached to an idea.
type Resource struct {
	ID      string    `json:"id"`
	IdeaID  string    `json:"idea_id"`
	URL     string    `json:"url"`
	Title   string    `json:"title"`
	Kind    string    `json:"kind"`
	Note    string    `json:"note"`
	Created time.Time `json:"created"`
}

// IdeaPatch is a partial update for an idea. Nil fields mean "unchanged".
type IdeaPatch struct {
	Title *string
	Body  *string
	Tags  *[]string
}

// PersonaPatch is a partial update for a persona. Nil fields mean "unchanged".
// Version is required by the service on every edit and must differ from the
// current version, so every change bumps the version captured by evaluations.
type PersonaPatch struct {
	Name         *string
	SystemPrompt *string
	Weight       *float64
	Version      *string
}
