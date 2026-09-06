package engine

// Persona represents a single AI reviewer that breaks down an idea.
type Persona struct {
	Name         string
	SystemPrompt string
}

// Result captures a single persona's structured output (or its error).
type Result struct {
	Persona   Persona
	Breakdown Breakdown
	Err       error
}
