package app

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// mockLLMAdapter is a mock implementation of LLMAdapter for testing
type mockLLMAdapter struct {
	insights []Insight
	err      error
}

func (m *mockLLMAdapter) Analyze(noteText string, types []TypeDefinition) ([]Insight, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.insights, nil
}

func TestProcessNote(t *testing.T) {
	// Setup: Create a temporary directory for test data
	tempDir, err := os.MkdirTemp("", "worker_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override the notesDir and dbPath for testing
	originalNotesDir := notesDir
	originalDBPath := dbPath
	defer func() {
		notesDir = originalNotesDir
		dbPath = originalDBPath
	}()

	// Use temporary paths
	testNotesDir := filepath.Join(tempDir, "notes")
	testDBPath := filepath.Join(tempDir, "events.db")

	// Create a note file for testing
	noteID := "20251120-143022-a1b2c3d4"
	noteContent := "Remember to call the dentist tomorrow. Also, I've been thinking a lot about health lately."

	if err := os.MkdirAll(testNotesDir, 0755); err != nil {
		t.Fatalf("Failed to create test notes directory: %v", err)
	}

	noteFilePath := filepath.Join(testNotesDir, noteID+".txt")
	if err := os.WriteFile(noteFilePath, []byte(noteContent), 0644); err != nil {
		t.Fatalf("Failed to write test note: %v", err)
	}

	// Temporarily override package-level constants for testing
	// Note: This is a workaround - in production code, you might want to make these configurable
	t.Setenv("TEST_NOTES_DIR", testNotesDir)
	t.Setenv("TEST_DB_PATH", testDBPath)

	// Initialize test database
	db, err := sql.Open("sqlite", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create events table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp INTEGER NOT NULL,
		event_type TEXT NOT NULL,
		source_note_id TEXT,
		payload_json TEXT
	);
	`
	if _, err := db.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create events table: %v", err)
	}

	// Create mock LLM adapter with test insights
	mockInsights := []Insight{
		{
			Type: "Task",
			Payload: map[string]interface{}{
				"description": "call the dentist",
				"priority":    "high",
			},
		},
		{
			Type: "Theme",
			Payload: map[string]interface{}{
				"name":        "Health",
				"description": "Focus on fitness and wellness",
			},
		},
	}

	mockAdapter := &mockLLMAdapter{
		insights: mockInsights,
		err:      nil,
	}

	// Test ProcessNote with mock data
	// Note: We need to temporarily change the package-level notesDir
	// This is a limitation of the current implementation
	oldNotesDir := notesDir
	notesDir = testNotesDir
	defer func() { notesDir = oldNotesDir }()

	err = ProcessNote(db, noteID, mockAdapter)
	if err != nil {
		t.Fatalf("ProcessNote failed: %v", err)
	}

	// Verify events were inserted
	rows, err := db.Query("SELECT event_type, source_note_id, payload_json FROM events ORDER BY id")
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}
	defer rows.Close()

	var events []struct {
		EventType    string
		SourceNoteID string
		PayloadJSON  string
	}

	for rows.Next() {
		var event struct {
			EventType    string
			SourceNoteID string
			PayloadJSON  string
		}
		if err := rows.Scan(&event.EventType, &event.SourceNoteID, &event.PayloadJSON); err != nil {
			t.Fatalf("Failed to scan event row: %v", err)
		}
		events = append(events, event)
	}

	// Verify we got 2 events (TaskDiscovered and ThemeIdentified)
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Verify first event (TaskDiscovered)
	if events[0].EventType != "TaskDiscovered" {
		t.Errorf("Expected first event type to be TaskDiscovered, got %s", events[0].EventType)
	}
	if events[0].SourceNoteID != noteID {
		t.Errorf("Expected source_note_id to be %s, got %s", noteID, events[0].SourceNoteID)
	}

	var taskPayload map[string]interface{}
	if err := json.Unmarshal([]byte(events[0].PayloadJSON), &taskPayload); err != nil {
		t.Fatalf("Failed to unmarshal task payload: %v", err)
	}
	if taskPayload["description"] != "call the dentist" {
		t.Errorf("Expected task description to be 'call the dentist', got %s", taskPayload["description"])
	}

	// Verify second event (ThemeIdentified)
	if events[1].EventType != "ThemeIdentified" {
		t.Errorf("Expected second event type to be ThemeIdentified, got %s", events[1].EventType)
	}
	if events[1].SourceNoteID != noteID {
		t.Errorf("Expected source_note_id to be %s, got %s", noteID, events[1].SourceNoteID)
	}

	var themePayload map[string]interface{}
	if err := json.Unmarshal([]byte(events[1].PayloadJSON), &themePayload); err != nil {
		t.Fatalf("Failed to unmarshal theme payload: %v", err)
	}
	if themePayload["name"] != "Health" {
		t.Errorf("Expected theme name to be 'Health', got %s", themePayload["name"])
	}
}

func TestProcessNote_ReadNoteError(t *testing.T) {
	// Initialize test database
	tempDir, err := os.MkdirTemp("", "worker_test_error")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testDBPath := filepath.Join(tempDir, "events.db")
	db, err := sql.Open("sqlite", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	mockAdapter := &mockLLMAdapter{}

	// Try to process a non-existent note
	err = ProcessNote(db, "nonexistent-note", mockAdapter)
	if err == nil {
		t.Error("Expected error when reading non-existent note, got nil")
	}
}
