## Assistant Project Overview

**System Overview:** Event-sourced note-taking system where raw notes are processed by an LLM and generate atomic insights which are stored in an append-only event stream.

**Data Model:**
- Raw notes: immutable text files, one per note (filename = UUID or timestamp)
- Event stream: SQLite database, append-only
- Events: NoteCreated, TaskDiscovered, ThemeIdentified
- Each event: id, timestamp, event_type, source_note_id, payload (JSON)

**Type System (Phase 1):**
- Types define what insights can be extracted (Task, Theme)
- Hardcoded as structs/constants in Go code for Phase 1
- Each type has: name, description (for LLM), schema (Go struct)
- Type definitions are passed to LLM during analysis
- LLM returns insights matching these type schemas
- Each insight becomes an event: event_type = type name, payload = JSON of schema fields
- Example: TaskDiscovered event has payload `{"description": "call dentist", "priority": "high"}`
- Types guide both extraction (what LLM looks for) and storage (event structure)

**LLM Integration:**
- Single adapter for one provider (OpenAI or Anthropic)
- Analysis prompt includes type definitions as "tools" or structured output schema
- LLM returns JSON array of insights matching defined types
- No error handling sophistication needed yet

**Scope Limits:**
- No queue (analysis runs immediately)
- No projections (read raw from DB)
- No superseding logic
- No dynamic types
- CLI only, no server
