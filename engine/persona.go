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
