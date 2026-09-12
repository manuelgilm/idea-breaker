package engine

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"aibreak/internal/domain"
)

// errUnparseable marks output that does not conform to the evaluation JSON
// schema (non-JSON, missing/extra keys, non-integer, or out-of-range score).
var errUnparseable = errors.New("invalid evaluation output")

// errUnparseableSynthesis marks synthesizer output that does not conform to
// the synthesis JSON schema (non-JSON, missing/extra keys, unknown verdict,
// or empty summary).
var errUnparseableSynthesis = errors.New("invalid synthesis output")

// validVerdicts is the fixed synthesizer verdict set.
var validVerdicts = map[string]bool{
	domain.VerdictProceed:      true,
	domain.VerdictPromising:    true,
	domain.VerdictRisky:        true,
	domain.VerdictPass:         true,
	domain.VerdictInconclusive: true,
}

// parseEvaluation strictly parses the provider content against the evaluation
// JSON schema: {"score": <int 0..5>, "rationale": <string>}.
func parseEvaluation(content string) (int, string, error) {
	var out struct {
		Score     *int   `json:"score"`
		Rationale string `json:"rationale"`
	}

	dec := json.NewDecoder(strings.NewReader(content))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return 0, "", errUnparseable
	}
	// Reject trailing content after the top-level object.
	if _, err := dec.Token(); err != io.EOF {
		return 0, "", errUnparseable
	}
	if out.Score == nil {
		return 0, "", errUnparseable
	}
	if *out.Score < 0 || *out.Score > 5 {
		return 0, "", errUnparseable
	}
	return *out.Score, out.Rationale, nil
}

// parseSynthesis strictly parses the provider content against the synthesis
// JSON schema: {"verdict": <one of the fixed labels>, "summary": <string>}.
func parseSynthesis(content string) (string, string, error) {
	var out struct {
		Verdict *string `json:"verdict"`
		Summary string  `json:"summary"`
	}

	dec := json.NewDecoder(strings.NewReader(content))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return "", "", errUnparseableSynthesis
	}
	// Reject trailing content after the top-level object.
	if _, err := dec.Token(); err != io.EOF {
		return "", "", errUnparseableSynthesis
	}
	if out.Verdict == nil || !validVerdicts[*out.Verdict] {
		return "", "", errUnparseableSynthesis
	}
	if strings.TrimSpace(out.Summary) == "" {
		return "", "", errUnparseableSynthesis
	}
	return *out.Verdict, out.Summary, nil
}
