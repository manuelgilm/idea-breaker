package persona

import "github.com/manuelgilm/idea-breaker/engine"

// Source yields the personas to use for a break. This is the seam to plug in a
// future prompt registry; the engine only ever sees []engine.Persona.
type Source interface {
	Load() ([]engine.Persona, error)
}
