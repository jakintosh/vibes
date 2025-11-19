package app

// LLMAdapter defines the interface for interacting with an LLM provider
type LLMAdapter interface {
	// Analyze processes the note text using the provided type definitions
	// and returns a list of extracted insights.
	Analyze(noteText string, types []TypeDefinition) ([]Insight, error)
}
