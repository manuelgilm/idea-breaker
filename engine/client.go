package engine

import "context"

// Caller makes a single API call for one persona.
// The real HTTP implementation (LLM provider) will be added later;
// the engine depends on this interface so it can be tested with a stub.
type Caller interface {
	Break(ctx context.Context, p Persona, idea, apiKey string) (string, error)
}

// StubCaller returns placeholder text without making any API call.
// A retry policy, when implemented, belongs here.
type StubCaller struct{}

// Break returns a placeholder response for the persona.
func (StubCaller) Break(ctx context.Context, p Persona, idea, apiKey string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return "placeholder response from " + p.Name + " for idea: " + idea, nil
	}
}
