## Assistant Project Overview

**System Overview:** Event-sourced note-taking system where raw notes are processed by an LLM and generate atomic insights which are stored in an append-only stream.

**Data Model:**
- Raw notes: immutable text files, one per note (filename = UUID or timestamp)
- Event stream: SQLite database, append-only
- Events: NoteCreated, TaskDiscovered, ThemeIdentified
- Each event: id, timestamp, event_type, source_note_id, payload (JSON)

**Type System (Phase 1):**
- Fixed types only: Task and Theme
- Hardcoded definitions in code (description, schema)
- Task schema: {description: string, priority?: string}
- Theme schema: {name: string, description: string}

**LLM Integration:**
- Single adapter for one provider (OpenAI or Anthropic)
- Analysis prompt includes type definitions
- LLM returns JSON array of insights
- No error handling sophistication needed yet

**Scope Limits:**
- No queue (analysis runs immediately)
- No projections (read raw from DB)
- No superseding logic
- No dynamic types
- CLI only, no server
