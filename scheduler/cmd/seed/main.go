package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Copy of db types to avoid import cycles or dependency issues if running standalone
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
	Dates        []EventDate `json:"dates"`
	Status       EventStatus `json:"status"`
	AcceptedDate *EventDate  `json:"accepted_date,omitempty"`
}

var (
	dbPath         = flag.String("db", "events.db", "Path to sqlite database")
	eventsPerWeek  = flag.Int("rate", 8, "Average events per week")
	acceptanceRate = flag.Float64("acceptance", 0.75, "Percentage of events to accept (0.0-1.0)")
)

func main() {
	flag.Parse()

	// Remove existing DB
	os.Remove(*dbPath)

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

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

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatal(err)
	}

	// Seed Data Configuration
	startDate := time.Date(2025, 11, 1, 0, 0, 0, 0, time.Local)
	endDate := time.Date(2025, 12, 31, 23, 59, 59, 0, time.Local)

	totalWeeks := int(endDate.Sub(startDate).Hours() / 24 / 7)
	totalEvents := totalWeeks * *eventsPerWeek

	log.Printf("Generating ~%d events between %s and %s...", totalEvents, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	descriptions := []string{
		"Team Sync", "Project Kickoff", "Client Meeting", "Workshop", "Training Session",
		"Board Meeting", "Community Gathering", "Music Rehearsal", "Tech Talk", "Hackathon",
		"Networking Event", "Product Launch", "Strategy Session", "Design Review", "Code Review",
	}
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Evan", "Fiona", "George", "Hannah"}

	// Track accepted intervals to generate conflicts
	var acceptedIntervals []EventDate

	for i := 0; i < totalEvents; i++ {
		// Random date within range
		daysRange := int(endDate.Sub(startDate).Hours() / 24)
		randomDay := rand.Intn(daysRange)
		eventStartDay := startDate.AddDate(0, 0, randomDay)

		// Random time (8am to 8pm start)
		startHour := 8 + rand.Intn(12)
		startMin := rand.Intn(4) * 15 // 0, 15, 30, 45

		start := time.Date(eventStartDay.Year(), eventStartDay.Month(), eventStartDay.Day(), startHour, startMin, 0, 0, time.Local)

		// Duration 1-8 hours
		durationHours := 1 + rand.Intn(8)
		end := start.Add(time.Duration(durationHours) * time.Hour)

		// Determine Status
		isAccepted := rand.Float64() < *acceptanceRate
		status := StatusRequested
		if isAccepted {
			status = StatusAccepted
		}

		// Create Event
		e := Event{
			ID:           uuid.New().String(),
			ContactName:  names[rand.Intn(len(names))],
			ContactPhone: "555-01" + fmt.Sprintf("%02d", rand.Intn(99)),
			ContactEmail: fmt.Sprintf("user%d@example.com", rand.Intn(100)),
			Description:  descriptions[rand.Intn(len(descriptions))],
			NeedsAV:      rand.Intn(2) == 0,
			Status:       status,
		}

		// Dates
		// If accepted, we have one accepted date.
		// If requested, we might have conflicts.

		if status == StatusAccepted {
			// Ensure no overlap with existing accepted events (simple retry logic)
			conflict := false
			for _, interval := range acceptedIntervals {
				if start.Before(interval.End) && end.After(interval.Start) {
					conflict = true
					break
				}
			}

			if conflict {
				// Skip this one or try again? Let's just skip to keep it simple
				continue
			}

			e.AcceptedDate = &EventDate{Start: start, End: end}
			e.Dates = []EventDate{{Start: start, End: end}} // Request matches acceptance
			acceptedIntervals = append(acceptedIntervals, *e.AcceptedDate)
		} else {
			// Requested
			// 50% chance to conflict with an existing accepted event
			if len(acceptedIntervals) > 0 && rand.Float64() < 0.5 {
				// Pick a random accepted event and overlap it
				target := acceptedIntervals[rand.Intn(len(acceptedIntervals))]
				// Shift slightly
				offset := rand.Intn(2) - 1 // -1, 0, 1 hour
				start = target.Start.Add(time.Duration(offset) * time.Hour)
				end = start.Add(time.Duration(durationHours) * time.Hour)
			}

			e.Dates = []EventDate{{Start: start, End: end}}
			// Add 1-2 alternate dates
			if rand.Intn(2) == 0 {
				altStart := start.AddDate(0, 0, 1+rand.Intn(3)) // 1-3 days later
				altEnd := altStart.Add(time.Duration(durationHours) * time.Hour)
				e.Dates = append(e.Dates, EventDate{Start: altStart, End: altEnd})
			}
		}

		// Insert
		datesJSON, _ := json.Marshal(e.Dates)
		var acceptedDateJSON sql.NullString
		if e.AcceptedDate != nil {
			b, _ := json.Marshal(e.AcceptedDate)
			acceptedDateJSON.String = string(b)
			acceptedDateJSON.Valid = true
		}

		_, err := db.Exec(`INSERT INTO events (id, contact_name, contact_phone, contact_email, description, needs_av, dates, status, accepted_date) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.ContactName, e.ContactPhone, e.ContactEmail, e.Description, e.NeedsAV, string(datesJSON), e.Status, acceptedDateJSON)
		if err != nil {
			log.Printf("Error inserting event: %v", err)
		}
	}

	log.Println("Seed complete.")
}
