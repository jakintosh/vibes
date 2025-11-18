package main

import (
	"fmt"
	"os"

	"assistant/internal/app"
	"git.sr.ht/~jakintosh/command-go/pkg/args"
)

var Add = &args.Command{
	Name: "add",
	Help: "Add a new note from text or file",
	Options: []args.Option{
		{
			Long: "file",
			Type: args.OptionTypeFlag,
			Help: "treat operand as file path instead of raw text",
		},
	},
	Operands: []args.Operand{
		{
			Name: "text",
			Help: "note content or file path (if --file is set)",
		},
	},
	Handler: func(input *args.Input) error {
		text := input.GetOperand("text")
		isFile := input.GetFlag("file")

		var content string

		if isFile {
			// Read from file
			fileContent, err := os.ReadFile(text)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			content = string(fileContent)
		} else {
			// Use text directly
			content = text
		}

		// Create note file
		noteID, err := app.CreateNote(content)
		if err != nil {
			return fmt.Errorf("failed to create note: %w", err)
		}

		// Initialize database and insert NoteCreated event
		db, err := app.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		if err := app.InsertNoteCreatedEvent(db, noteID); err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}

		fmt.Printf("Note created with ID: %s\n", noteID)
		return nil
	},
}
