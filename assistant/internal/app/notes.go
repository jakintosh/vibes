package app

import (
	"crypto/rand"
	"encoding/hex"
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

	// Generate random suffix for uniqueness (4 bytes = 8 hex chars)
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random suffix: %w", err)
	}
	suffix := hex.EncodeToString(randomBytes)

	// Generate timestamp-based filename with uniqueness suffix (YYYYMMDD-HHMMSS-XXXXXXXX.txt)
	timestamp := time.Now().Format("20060102-150405")
	noteID := fmt.Sprintf("%s-%s", timestamp, suffix)
	filename := fmt.Sprintf("%s.txt", noteID)
	filepath := filepath.Join(notesDir, filename)

	// Write the note content to file
	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write note: %w", err)
	}

	return noteID, nil
}

// ReadNote reads a note's content from disk by its ID
func ReadNote(noteID string) (string, error) {
	filename := fmt.Sprintf("%s.txt", noteID)
	filepath := filepath.Join(notesDir, filename)

	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read note %s: %w", noteID, err)
	}

	return string(content), nil
}
