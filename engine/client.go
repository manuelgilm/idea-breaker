package engine

import "context"

// Caller makes a single API call for one persona.
// The real HTTP implementation (LLM provider) will be added later;
// the engine depends on this interface so it can be tested with a stub.
// It is intentionally free of any auth concern; credentials are bound to the
// concrete caller at construction time.
type Caller interface {
	Break(ctx context.Context, p Persona, idea string) (string, error)
}

// StubCaller returns placeholder text without making any API call.
// A retry policy, when implemented, belongs here.
type StubCaller struct{}

// Break returns a placeholder response for the persona.
func (StubCaller) Break(ctx context.Context, p Persona, idea string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return "placeholder response from " + p.Name + " for idea: " + idea, nil
	}
}

// HTTPCaller makes real LLM HTTP calls. It is still a placeholder; the actual
// request/response plumbing comes later.
type HTTPCaller struct {
	APIKey string
}

// Break returns a placeholder response for the persona.
func (HTTPCaller) Break(ctx context.Context, p Persona, idea string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return "placeholder response from " + p.Name + " for idea: " + idea, nil
	}
}
