package app

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	dbPath = "./data/events.db"
)

// InitDB initializes the SQLite database and creates the events table with indexes
func InitDB() (*sql.DB, error) {
	// Ensure the data directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Open/create the database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create the events table
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
		db.Close()
		return nil, fmt.Errorf("failed to create events table: %w", err)
	}

	// Create index on event_type
	createEventTypeIndexSQL := `
	CREATE INDEX IF NOT EXISTS idx_event_type ON events(event_type);
	`

	if _, err := db.Exec(createEventTypeIndexSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create event_type index: %w", err)
	}

	// Create index on source_note_id
	createSourceNoteIndexSQL := `
	CREATE INDEX IF NOT EXISTS idx_source_note_id ON events(source_note_id);
	`

	if _, err := db.Exec(createSourceNoteIndexSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create source_note_id index: %w", err)
	}

	return db, nil
}
