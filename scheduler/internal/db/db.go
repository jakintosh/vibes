package db

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite"
)

type EventStatus string

const (
	StatusRequested EventStatus = "Requested"
	StatusAccepted  EventStatus = "Accepted"
)

type EventDate struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Event struct {
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	ContactName  string      `json:"contact_name"`
	ContactPhone string      `json:"contact_phone"`
	ContactEmail string      `json:"contact_email"`
	Description  string      `json:"description"`
	NeedsAV      bool        `json:"needs_av"`
	Dates        []EventDate `json:"dates"` // Stored as JSON
	Status       EventStatus `json:"status"`
	AcceptedDate *EventDate  `json:"accepted_date,omitempty"` // Stored as JSON
	CreatedAt    time.Time   `json:"created_at"`
}

var db *sql.DB

func InitDB(dataSourceName string) error {
	var err error
	db, err = sql.Open("sqlite", dataSourceName)
	if err != nil {
		return err
	}

	createTableSQL := `CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		title TEXT,
		contact_name TEXT,
		contact_phone TEXT,
		contact_email TEXT,
		description TEXT,
		needs_av BOOLEAN,
		dates TEXT,
		status TEXT,
		accepted_date TEXT,
		created_at DATETIME
	);`

	_, err = db.Exec(createTableSQL)
	return err
}

func Close() error {
	return db.Close()
}

func CreateEvent(e Event) error {
	datesJSON, err := json.Marshal(e.Dates)
	if err != nil {
		return err
	}

	// Ensure CreatedAt is set
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	_, err = db.Exec(`INSERT INTO events (id, title, contact_name, contact_phone, contact_email, description, needs_av, dates, status, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Title, e.ContactName, e.ContactPhone, e.ContactEmail, e.Description, e.NeedsAV, string(datesJSON), e.Status, e.CreatedAt)
	return err
}

func GetEvents() ([]Event, error) {
	rows, err := db.Query("SELECT id, title, contact_name, contact_phone, contact_email, description, needs_av, dates, status, accepted_date, created_at FROM events")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var datesStr string
		var acceptedDateStr sql.NullString
		var createdAt sql.NullTime

		err := rows.Scan(&e.ID, &e.Title, &e.ContactName, &e.ContactPhone, &e.ContactEmail, &e.Description, &e.NeedsAV, &datesStr, &e.Status, &acceptedDateStr, &createdAt)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(datesStr), &e.Dates); err != nil {
			return nil, err
		}

		if acceptedDateStr.Valid && acceptedDateStr.String != "" {
			var ad EventDate
			if err := json.Unmarshal([]byte(acceptedDateStr.String), &ad); err != nil {
				return nil, err
			}
			e.AcceptedDate = &ad
		}

		if createdAt.Valid {
			e.CreatedAt = createdAt.Time
		}

		events = append(events, e)
	}
	return events, nil
}

func GetEvent(id string) (*Event, error) {
	var e Event
	var datesStr string
	var acceptedDateStr sql.NullString
	var createdAt sql.NullTime

	err := db.QueryRow("SELECT id, title, contact_name, contact_phone, contact_email, description, needs_av, dates, status, accepted_date, created_at FROM events WHERE id = ?", id).Scan(
		&e.ID, &e.Title, &e.ContactName, &e.ContactPhone, &e.ContactEmail, &e.Description, &e.NeedsAV, &datesStr, &e.Status, &acceptedDateStr, &createdAt)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(datesStr), &e.Dates); err != nil {
		return nil, err
	}

	if acceptedDateStr.Valid && acceptedDateStr.String != "" {
		var ad EventDate
		if err := json.Unmarshal([]byte(acceptedDateStr.String), &ad); err != nil {
			return nil, err
		}
		e.AcceptedDate = &ad
	}

	if createdAt.Valid {
		e.CreatedAt = createdAt.Time
	}

	return &e, nil
}

func AcceptEvent(id string, date EventDate) error {
	dateJSON, err := json.Marshal(date)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE events SET status = ?, accepted_date = ? WHERE id = ?", StatusAccepted, string(dateJSON), id)
	return err
}
