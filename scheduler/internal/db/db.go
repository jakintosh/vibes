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
	ContactName  string      `json:"contact_name"`
	ContactPhone string      `json:"contact_phone"`
	ContactEmail string      `json:"contact_email"`
	Description  string      `json:"description"`
	NeedsAV      bool        `json:"needs_av"`
	Dates        []EventDate `json:"dates"` // Stored as JSON
	Status       EventStatus `json:"status"`
	AcceptedDate *EventDate  `json:"accepted_date,omitempty"` // Stored as JSON
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
		contact_name TEXT,
		contact_phone TEXT,
		contact_email TEXT,
		description TEXT,
		needs_av BOOLEAN,
		dates TEXT,
		status TEXT,
		accepted_date TEXT
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

	_, err = db.Exec(`INSERT INTO events (id, contact_name, contact_phone, contact_email, description, needs_av, dates, status) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.ContactName, e.ContactPhone, e.ContactEmail, e.Description, e.NeedsAV, string(datesJSON), e.Status)
	return err
}

func GetEvents() ([]Event, error) {
	rows, err := db.Query("SELECT id, contact_name, contact_phone, contact_email, description, needs_av, dates, status, accepted_date FROM events")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var datesStr string
		var acceptedDateStr sql.NullString

		err := rows.Scan(&e.ID, &e.ContactName, &e.ContactPhone, &e.ContactEmail, &e.Description, &e.NeedsAV, &datesStr, &e.Status, &acceptedDateStr)
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

		events = append(events, e)
	}
	return events, nil
}

func GetEvent(id string) (*Event, error) {
	var e Event
	var datesStr string
	var acceptedDateStr sql.NullString

	err := db.QueryRow("SELECT id, contact_name, contact_phone, contact_email, description, needs_av, dates, status, accepted_date FROM events WHERE id = ?", id).Scan(
		&e.ID, &e.ContactName, &e.ContactPhone, &e.ContactEmail, &e.Description, &e.NeedsAV, &datesStr, &e.Status, &acceptedDateStr)
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
