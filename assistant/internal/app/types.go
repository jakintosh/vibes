package app

// TypeRegistry holds the in-memory type definitions
var TypeRegistry = map[string]TypeDefinition{
	"Task": {
		Name:        "Task",
		Description: "A specific action item or todo. Include description and optional priority.",
		Schema:      Task{},
	},
	"Theme": {
		Name:        "Theme",
		Description: "A recurring topic or concept. Include name and brief description.",
		Schema:      Theme{},
	},
}

// TypeDefinition represents metadata about a registered type
type TypeDefinition struct {
	Name        string
	Description string
	Schema      interface{}
}

// Task represents a specific action item or todo
// Include description and optional priority
type Task struct {
	Description string `json:"description"`
	Priority    string `json:"priority,omitempty"` // optional: e.g., "high", "medium", "low"
}

// Theme represents a recurring topic or concept
// Include name and brief description
type Theme struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Insight represents a structured piece of intelligence extracted from a note
type Insight struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}
