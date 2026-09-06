package engine

// Persona represents a single AI reviewer that breaks down an idea.
type Persona struct {
	Name         string
	SystemPrompt string
}

// Result captures a single persona's output (or its error).
type Result struct {
	Persona  Persona
	Response string
	Err      error
}

// BuiltInPersonas returns the default set of personas.
// This is the constructor to extend when adding more personas.
func BuiltInPersonas() []Persona {
	return []Persona{
		{Name: "pessimist", SystemPrompt: "You are a pessimist. Point out every reason why this idea will fail."},
		{Name: "optimist", SystemPrompt: "You are an optimist. Highlight every reason why this idea will succeed."},
		{Name: "architect", SystemPrompt: "You are an architect. Describe how this idea could be built."},
	}
}
