package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// ProcessNote analyzes a note and writes insights to the event stream
// It fetches the note text, calls the LLM adapter, and writes each insight as a separate event
func ProcessNote(db *sql.DB, noteID string, llmAdapter LLMAdapter) error {
	// Fetch note text from file
	noteText, err := ReadNote(noteID)
	if err != nil {
		return fmt.Errorf("failed to read note: %w", err)
	}

	// Get type definitions from the registry
	var typeDefinitions []TypeDefinition
	for _, typeDef := range TypeRegistry {
		typeDefinitions = append(typeDefinitions, typeDef)
	}

	// Call LLM adapter with note and type definitions
	insights, err := llmAdapter.Analyze(noteText, typeDefinitions)
	if err != nil {
		return fmt.Errorf("failed to analyze note: %w", err)
	}

	// Parse returned insights and write to events table as separate events
	for _, insight := range insights {
		// Marshal payload to JSON
		payloadJSON, err := json.Marshal(insight.Payload)
		if err != nil {
			return fmt.Errorf("failed to marshal insight payload: %w", err)
		}

		// Write insight event to database (each insight event links to source_note_id)
		if err := InsertInsightEvent(db, noteID, insight, string(payloadJSON)); err != nil {
			return fmt.Errorf("failed to insert insight event: %w", err)
		}
	}

	return nil
}
