package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	notesDir = "./data/notes"
)

// CreateNote writes a note to disk with a timestamp-based filename and returns the note ID
func CreateNote(content string) (string, error) {
	// Ensure the notes directory exists
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create notes directory: %w", err)
	}

	// Generate timestamp-based filename (YYYYMMDD-HHMMSS.txt)
	noteID := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s.txt", noteID)
	filepath := filepath.Join(notesDir, filename)

	// Write the note content to file
	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write note: %w", err)
	}

	return noteID, nil
}
