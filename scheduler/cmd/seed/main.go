package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"event-scheduler/internal/db"

	"github.com/google/uuid"
)

var (
	dbPath         = flag.String("db", "events.db", "Path to sqlite database")
	eventsPerWeek  = flag.Int("rate", 8, "Average events per week")
	acceptanceRate = flag.Float64("acceptance", 0.75, "Percentage of events to accept (0.0-1.0)")
)

func main() {
	flag.Parse()

	if err := os.Remove(*dbPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed removing db: %v", err)
	}

	if err := db.InitDB(*dbPath); err != nil {
		log.Fatalf("failed initializing db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("error closing db: %v", err)
		}
	}()

	startDate := time.Date(2025, 11, 1, 0, 0, 0, 0, time.Local)
	endDate := time.Date(2025, 12, 31, 23, 59, 59, 0, time.Local)

	totalWeeks := int(endDate.Sub(startDate).Hours() / 24 / 7)
	totalEvents := totalWeeks * *eventsPerWeek

	log.Printf("Generating ~%d events between %s and %s...", totalEvents, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	titles := []string{
		"Team Sync", "Project Kickoff", "Client Meeting", "Workshop", "Training Session",
		"Board Meeting", "Community Gathering", "Music Rehearsal", "Tech Talk", "Hackathon",
		"Networking Event", "Product Launch", "Strategy Session", "Design Review", "Code Review",
	}
	descriptions := []string{
		"A regular sync to discuss project status and blockers.",
		"Kickoff meeting for the new Q4 initiative.",
		"Meeting with the client to review requirements.",
		"Interactive workshop on new technologies.",
		"Training session for new hires.",
		"Quarterly board meeting.",
		"Gathering for the local community.",
		"Rehearsal for the upcoming concert.",
		"Technical talk about Go concurrency.",
		"Weekend hackathon for innovation.",
		"Networking event for local professionals.",
		"Launch event for our new product line.",
		"Strategic planning for the next year.",
		"Review of the new design system.",
		"Code review session for the core module.",
	}
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Evan", "Fiona", "George", "Hannah"}

	var acceptedIntervals []db.EventDate

	for range totalEvents {
		daysRange := int(endDate.Sub(startDate).Hours() / 24)
		randomDay := rand.Intn(daysRange)
		eventStartDay := startDate.AddDate(0, 0, randomDay)

		startHour := 8 + rand.Intn(12)
		startMin := rand.Intn(4) * 15

		start := time.Date(eventStartDay.Year(), eventStartDay.Month(), eventStartDay.Day(), startHour, startMin, 0, 0, time.Local)

		durationHours := 1 + rand.Intn(8)
		end := start.Add(time.Duration(durationHours) * time.Hour)

		isAccepted := rand.Float64() < *acceptanceRate
		status := db.StatusRequested
		if isAccepted {
			status = db.StatusAccepted
		}

		titleIndex := rand.Intn(len(titles))
		e := db.Event{
			ID:           uuid.New().String(),
			Title:        titles[titleIndex],
			ContactName:  names[rand.Intn(len(names))],
			ContactPhone: "555-01" + fmt.Sprintf("%02d", rand.Intn(99)),
			ContactEmail: fmt.Sprintf("user%d@example.com", rand.Intn(100)),
			Description:  descriptions[titleIndex],
			NeedsAV:      rand.Intn(2) == 0,
			Status:       status,
			CreatedAt:    start.AddDate(0, 0, -1-rand.Intn(30)),
		}

		if status == db.StatusAccepted {
			conflict := false
			for _, interval := range acceptedIntervals {
				if start.Before(interval.End) && end.After(interval.Start) {
					conflict = true
					break
				}
			}

			if conflict {
				continue
			}

			e.AcceptedDate = &db.EventDate{Start: start, End: end}
			e.Dates = []db.EventDate{{Start: start, End: end}}
			acceptedIntervals = append(acceptedIntervals, *e.AcceptedDate)
		} else {
			if len(acceptedIntervals) > 0 && rand.Float64() < 0.5 {
				target := acceptedIntervals[rand.Intn(len(acceptedIntervals))]
				offset := rand.Intn(2) - 1
				start = target.Start.Add(time.Duration(offset) * time.Hour)
				end = start.Add(time.Duration(durationHours) * time.Hour)
			}

			e.Dates = []db.EventDate{{Start: start, End: end}}
			if rand.Intn(2) == 0 {
				altStart := start.AddDate(0, 0, 1+rand.Intn(3))
				altEnd := altStart.Add(time.Duration(durationHours) * time.Hour)
				e.Dates = append(e.Dates, db.EventDate{Start: altStart, End: altEnd})
			}
		}

		if err := db.CreateEvent(e); err != nil {
			log.Printf("Error inserting event: %v", err)
		}
	}

	log.Println("Seed complete.")
}
