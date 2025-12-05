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
	StatusConfirmed EventStatus = "Confirmed"
	StatusWithdrawn EventStatus = "Withdrawn"
	StatusDenied    EventStatus = "Denied"
	StatusCanceled  EventStatus = "Canceled"
)

type PaymentStatus string

const (
	PaymentProposed   PaymentStatus = "Proposed"
	PaymentDue        PaymentStatus = "Due"
	PaymentPaid       PaymentStatus = "Paid"
	PaymentRefundable PaymentStatus = "Refundable"
	PaymentSettled    PaymentStatus = "Settled"
)

type EventDate struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Event struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	ContactName    string        `json:"contact_name"`
	ContactPhone   string        `json:"contact_phone"`
	ContactEmail   string        `json:"contact_email"`
	Description    string        `json:"description"`
	NeedsAV        bool          `json:"needs_av"`
	Dates          []EventDate   `json:"dates"` // Stored as JSON
	Status         EventStatus   `json:"status"`
	AcceptedDate   *EventDate    `json:"accepted_date,omitempty"` // Stored as JSON
	PaymentStatus  PaymentStatus `json:"payment_status"`
	ProposedCost   float64       `json:"proposed_cost"`
	DepositAmount  float64       `json:"deposit_amount"`
	AmountReceived float64       `json:"amount_received"`
	StaffOpener    string        `json:"staff_opener"`
	StaffCloser    string        `json:"staff_closer"`
	StaffNotes     string        `json:"staff_notes"`
	CreatedAt      time.Time     `json:"created_at"`
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
		payment_status TEXT,
		proposed_cost REAL DEFAULT 0,
		deposit_amount REAL DEFAULT 0,
		amount_received REAL DEFAULT 0,
		created_at DATETIME
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	migrations := []string{
		"ALTER TABLE events ADD COLUMN payment_status TEXT;",
		"ALTER TABLE events ADD COLUMN proposed_cost REAL DEFAULT 0;",
		"ALTER TABLE events ADD COLUMN deposit_amount REAL DEFAULT 0;",
		"ALTER TABLE events ADD COLUMN amount_received REAL DEFAULT 0;",
		"ALTER TABLE events ADD COLUMN staff_opener TEXT DEFAULT '';",
		"ALTER TABLE events ADD COLUMN staff_closer TEXT DEFAULT '';",
		"ALTER TABLE events ADD COLUMN staff_notes TEXT DEFAULT '';",
	}

	for _, m := range migrations {
		if _, execErr := db.Exec(m); execErr != nil {
			// Ignore duplicate column errors so init remains idempotent on existing DBs.
			continue
		}
	}

	return nil
}

func Close() error {
	return db.Close()
}

func CreateEvent(e Event) error {
	datesJSON, err := json.Marshal(e.Dates)
	if err != nil {
		return err
	}

	var acceptedDateJSON sql.NullString
	if e.AcceptedDate != nil {
		b, err := json.Marshal(e.AcceptedDate)
		if err != nil {
			return err
		}
		acceptedDateJSON = sql.NullString{String: string(b), Valid: true}
	}

	// Ensure CreatedAt is set
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	if e.PaymentStatus == "" {
		e.PaymentStatus = PaymentProposed
	}

	_, err = db.Exec(`INSERT INTO events (id, title, contact_name, contact_phone, contact_email, description, needs_av, dates, status, accepted_date, payment_status, proposed_cost, deposit_amount, amount_received, staff_opener, staff_closer, staff_notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Title, e.ContactName, e.ContactPhone, e.ContactEmail, e.Description, e.NeedsAV, string(datesJSON), e.Status, acceptedDateJSON, e.PaymentStatus, e.ProposedCost, e.DepositAmount, e.AmountReceived, e.StaffOpener, e.StaffCloser, e.StaffNotes, e.CreatedAt)
	return err
}

func GetEvents() ([]Event, error) {
	rows, err := db.Query("SELECT id, title, contact_name, contact_phone, contact_email, description, needs_av, dates, status, accepted_date, payment_status, proposed_cost, deposit_amount, amount_received, staff_opener, staff_closer, staff_notes, created_at FROM events")
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

		err := rows.Scan(&e.ID, &e.Title, &e.ContactName, &e.ContactPhone, &e.ContactEmail, &e.Description, &e.NeedsAV, &datesStr, &e.Status, &acceptedDateStr, &e.PaymentStatus, &e.ProposedCost, &e.DepositAmount, &e.AmountReceived, &e.StaffOpener, &e.StaffCloser, &e.StaffNotes, &createdAt)
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

		if e.PaymentStatus == "" {
			e.PaymentStatus = PaymentProposed
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

	err := db.QueryRow("SELECT id, title, contact_name, contact_phone, contact_email, description, needs_av, dates, status, accepted_date, payment_status, proposed_cost, deposit_amount, amount_received, staff_opener, staff_closer, staff_notes, created_at FROM events WHERE id = ?", id).Scan(
		&e.ID, &e.Title, &e.ContactName, &e.ContactPhone, &e.ContactEmail, &e.Description, &e.NeedsAV, &datesStr, &e.Status, &acceptedDateStr, &e.PaymentStatus, &e.ProposedCost, &e.DepositAmount, &e.AmountReceived, &e.StaffOpener, &e.StaffCloser, &e.StaffNotes, &createdAt)
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

	if e.PaymentStatus == "" {
		e.PaymentStatus = PaymentProposed
	}

	return &e, nil
}

func AcceptEvent(id string, date EventDate, proposedCost float64, deposit float64) error {
	dateJSON, err := json.Marshal(date)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE events SET status = ?, accepted_date = ?, payment_status = ?, proposed_cost = ?, deposit_amount = ?, amount_received = 0 WHERE id = ?", StatusAccepted, string(dateJSON), PaymentProposed, proposedCost, deposit, id)
	return err
}

func UpdateEventStatus(id string, status EventStatus) error {
	_, err := db.Exec("UPDATE events SET status = ? WHERE id = ?", status, id)
	return err
}

func UpdatePayment(id string, status PaymentStatus, amountReceived float64) error {
	_, err := db.Exec("UPDATE events SET payment_status = ?, amount_received = ? WHERE id = ?", status, amountReceived, id)
	return err
}

func UpdatePaymentStatus(id string, status PaymentStatus) error {
	_, err := db.Exec("UPDATE events SET payment_status = ? WHERE id = ?", status, id)
	return err
}

func UpdateStaffing(id, opener, closer, notes string) error {
	_, err := db.Exec("UPDATE events SET staff_opener = ?, staff_closer = ?, staff_notes = ? WHERE id = ?", opener, closer, notes, id)
	return err
}
