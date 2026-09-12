// Package engine implements pure idea evaluation and scoring. It depends only
// on the llm.Provider interface and performs no I/O itself.
package engine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"aibreak/internal/domain"
	"aibreak/internal/llm"
)

var (
	// ErrEmptyPersonas indicates no personas were provided for evaluation.
	ErrEmptyPersonas = errors.New("engine: no personas to evaluate")
	// ErrNoResults indicates every persona failed to produce a valid score.
	ErrNoResults = errors.New("engine: all personas failed")
	// ErrZeroWeight indicates all successful personas carry zero weight.
	ErrZeroWeight = errors.New("engine: all successful personas have zero weight")
)

// jsonInstruction is appended to every persona's system prompt to force a
// parseable, schema-strict JSON response.
const jsonInstruction = ` Respond with a single JSON object with exactly two keys: "score" (an integer from 0 to 5) and "rationale" (a short string). No other text.`

// synthesizerPrompt is the system prompt for the optional second-stage
// synthesis pass (see the "Synthesizer contract" in the product spec). It is a
// fixed instruction; per-persona rubric anchors do not apply here.
const synthesizerPrompt = `You are the SYNTHESIZER, the final aggregation stage. You are not a peer evaluator. Synthesize this idea's FeasibilityScore — deterministic total plus per-persona breakdown — into a single, cohesive, decision-useful verdict. Do not merely list what each persona said; combine themes into a unified analysis. Ground truth: base the verdict solely on the provided data; reference the deterministic total score; do not generate your own numeric scores, invent personas, or hallucinate. Summary must be strictly 3-5 sentences: (1) bottom-line verdict with the total; (2) core strengths; (3) most critical risks; (4/5) recommendation and, if any requested persona failed, an explicit acknowledgment of the incomplete coverage. Respond with a single JSON object with exactly two keys: "verdict" (one of proceed, promising, risky, pass, inconclusive) and "summary" (a short string). No other text.`

// Evaluator runs persona evaluations and aggregates their scores.
type Evaluator struct {
	provider    llm.Provider
	model       string
	temperature float64
	maxTokens   int
	retries     int
	timeout     time.Duration
	concurrency int
}

// Option configures an Evaluator.
type Option func(*Evaluator)

// WithModel sets the LLM model name.
func WithModel(m string) Option { return func(e *Evaluator) { e.model = m } }

// WithTemperature sets the sampling temperature.
func WithTemperature(t float64) Option { return func(e *Evaluator) { e.temperature = t } }

// WithMaxTokens sets the max completion tokens.
func WithMaxTokens(n int) Option { return func(e *Evaluator) { e.maxTokens = n } }

// WithRetries sets the number of retries per persona (beyond the first attempt).
func WithRetries(n int) Option { return func(e *Evaluator) { e.retries = n } }

// WithTimeout sets the per-call timeout.
func WithTimeout(d time.Duration) Option { return func(e *Evaluator) { e.timeout = d } }

// WithConcurrency sets the max number of concurrent persona evaluations.
func WithConcurrency(n int) Option { return func(e *Evaluator) { e.concurrency = n } }

// New creates an Evaluator with spec defaults.
func New(provider llm.Provider, opts ...Option) *Evaluator {
	e := &Evaluator{
		provider:    provider,
		model:       "gpt-4o-mini",
		temperature: 0,
		maxTokens:   512,
		retries:     1,
		timeout:     60 * time.Second,
		concurrency: 4,
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// Evaluate evaluates an idea against the given personas and returns an
// aggregated FeasibilityScore. It returns an error when no personas are given,
// when every persona fails, or when all successful personas have zero weight.
// When summarize is true, a best-effort second-stage synthesis pass populates
// Summary and Verdict; a failed synthesis leaves those fields empty without
// failing the run.
func (e *Evaluator) Evaluate(ctx context.Context, idea domain.Idea, personas []domain.Persona, summarize bool) (domain.FeasibilityScore, error) {
	if len(personas) == 0 {
		return domain.FeasibilityScore{}, ErrEmptyPersonas
	}

	runID := ulid.Make().String()
	now := time.Now().UTC()

	results := make([]domain.Evaluation, len(personas))
	sem := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup
	for i, p := range personas {
		wg.Add(1)
		go func(i int, p domain.Persona) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = e.evaluatePersona(ctx, runID, now, idea, p)
		}(i, p)
	}
	wg.Wait()

	score, err := e.aggregate(runID, now, idea.ID, results)
	if err != nil {
		return domain.FeasibilityScore{}, err
	}

	if summarize {
		e.synthesize(ctx, idea, &score)
	}
	return score, nil
}

// synthesize runs the optional single sequential synthesis pass. It is
// best-effort: any failure leaves Summary and Verdict empty.
func (e *Evaluator) synthesize(ctx context.Context, idea domain.Idea, score *domain.FeasibilityScore) {
	req := llm.Request{
		Model: e.model,
		Messages: []llm.Message{
			{Role: "system", Content: synthesizerPrompt},
			{Role: "user", Content: synthesisInput(idea, *score)},
		},
		Temperature: e.temperature,
		MaxTokens:   e.maxTokens,
		JSONMode:    true,
	}

	attempts := e.retries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, e.timeout)
		resp, err := e.provider.Complete(callCtx, req)
		cancel()

		if err == nil {
			verdict, summary, perr := parseSynthesis(resp.Content)
			if perr == nil {
				score.Verdict = verdict
				score.Summary = summary
				// The engine enforces coverage semantics deterministically:
				// partial runs are always inconclusive, regardless of the
				// model's judgment.
				if score.Responded < score.Requested {
					score.Verdict = domain.VerdictInconclusive
				}
				return
			}
			if attempt < attempts-1 {
				continue
			}
			return
		}

		if llm.IsRetryable(err) && attempt < attempts-1 {
			continue
		}
		return
	}
}

// synthesisInput renders the idea and aggregate for the synthesizer.
func synthesisInput(idea domain.Idea, score domain.FeasibilityScore) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Idea: %s\n", idea.Title)
	if idea.Body != "" {
		fmt.Fprintf(&b, "Description: %s\n", idea.Body)
	}
	fmt.Fprintf(&b, "Total: %.1f, Requested: %d, Responded: %d\n", score.Total, score.Requested, score.Responded)
	for _, ev := range score.Breakdown {
		if ev.Status == domain.StatusSuccess {
			fmt.Fprintf(&b, "- %s (score %d/5): %s\n", ev.PersonaID, ev.Score, ev.Rationale)
		} else {
			fmt.Fprintf(&b, "- %s: failed (%s)\n", ev.PersonaID, ev.Error)
		}
	}
	return b.String()
}

func (e *Evaluator) evaluatePersona(ctx context.Context, runID string, now time.Time, idea domain.Idea, p domain.Persona) domain.Evaluation {
	ev := domain.Evaluation{
		RunID:          runID,
		IdeaID:         idea.ID,
		PersonaID:      p.ID,
		PersonaVersion: p.Version,
		Weight:         p.Weight,
		Status:         domain.StatusFailed,
		Created:        now,
	}

	req := llm.Request{
		Model: e.model,
		Messages: []llm.Message{
			{Role: "system", Content: p.SystemPrompt + jsonInstruction},
			{Role: "user", Content: userMessage(idea)},
		},
		Temperature: e.temperature,
		MaxTokens:   e.maxTokens,
		JSONMode:    true,
	}

	attempts := e.retries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, e.timeout)
		resp, err := e.provider.Complete(callCtx, req)
		cancel()

		if err == nil {
			score, rationale, perr := parseEvaluation(resp.Content)
			if perr == nil {
				ev.Status = domain.StatusSuccess
				ev.Score = score
				ev.Rationale = rationale
				return ev
			}
			if attempt < attempts-1 {
				continue
			}
			ev.Error = "unparseable output: " + perr.Error()
			return ev
		}

		if llm.IsRetryable(err) && attempt < attempts-1 {
			continue
		}
		ev.Error = err.Error()
		return ev
	}
	return ev
}

func (e *Evaluator) aggregate(runID string, now time.Time, ideaID string, results []domain.Evaluation) (domain.FeasibilityScore, error) {
	breakdown := append([]domain.Evaluation(nil), results...)
	sort.Slice(breakdown, func(i, j int) bool { return breakdown[i].PersonaID < breakdown[j].PersonaID })

	score := domain.FeasibilityScore{
		RunID:     runID,
		IdeaID:    ideaID,
		Breakdown: breakdown,
		Requested: len(results),
		Created:   now,
	}

	var num, den float64
	responded := 0
	minScore, maxScore := 0, 0
	for _, ev := range breakdown {
		if ev.Status != domain.StatusSuccess {
			continue
		}
		responded++
		num += float64(ev.Score) * ev.Weight
		den += ev.Weight
		if responded == 1 || ev.Score < minScore {
			minScore = ev.Score
		}
		if responded == 1 || ev.Score > maxScore {
			maxScore = ev.Score
		}
	}
	score.Responded = responded
	score.Spread = float64(maxScore - minScore)

	if responded == 0 {
		return domain.FeasibilityScore{}, ErrNoResults
	}
	if den == 0 {
		return domain.FeasibilityScore{}, ErrZeroWeight
	}
	total := 100 * (num / (5 * den))
	score.Total = math.Round(total*10) / 10
	return score, nil
}

func userMessage(idea domain.Idea) string {
	if idea.Body == "" {
		return idea.Title
	}
	return idea.Title + "\n\n" + idea.Body
}

// Agreement maps a spread value (0–5) to its interpretation label, per the
// product spec §3 thresholds. It is the single source of truth for the label
// on every surface (CLI text, docs); API consumers use the numeric spread.
func Agreement(spread float64) string {
	switch {
	case spread <= 1:
		return "consensus"
	case spread <= 3:
		return "mixed"
	default:
		return "divided"
	}
}
