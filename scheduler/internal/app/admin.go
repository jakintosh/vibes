package app

import (
	"net/http"
	"sort"
	"time"

	db "event-scheduler/internal/db"
)

type AdminEventData struct {
	db.Event
	DateConflicts map[int]bool // Index of requested date -> isConflict
}

type AdminPageData struct {
	InboxEvents    []AdminEventData
	UpcomingEvents []AdminEventData
	SortMode       string
}

func HandleGetAdmin(w http.ResponseWriter, r *http.Request) {
	events, err := db.GetEvents()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	sortMode := r.URL.Query().Get("sort")
	if sortMode == "" {
		sortMode = "longest_waiting"
	}

	// Identify Accepted Events for conflict checking
	var acceptedEvents []db.EventDate
	for _, e := range events {
		if e.Status == db.StatusAccepted && e.AcceptedDate != nil {
			acceptedEvents = append(acceptedEvents, *e.AcceptedDate)
		}
	}

	var inbox []AdminEventData
	var upcoming []AdminEventData
	now := time.Now()

	for _, e := range events {
		data := AdminEventData{
			Event:         e,
			DateConflicts: make(map[int]bool),
		}

		if e.Status == db.StatusRequested {
			// Calculate conflicts
			for i, d := range e.Dates {
				for _, accepted := range acceptedEvents {
					if d.Start.Before(accepted.End) && d.End.After(accepted.Start) {
						data.DateConflicts[i] = true
						break
					}
				}
			}
			inbox = append(inbox, data)
		} else if e.Status == db.StatusAccepted && e.AcceptedDate != nil {
			// Only show future events in Upcoming
			if e.AcceptedDate.Start.After(now) {
				upcoming = append(upcoming, data)
			}
		}
	}

	// Sort Inbox
	if sortMode == "soonest" {
		// Sort by earliest requested date
		sort.Slice(inbox, func(i, j int) bool {
			startI := getEarliestDate(inbox[i].Dates)
			startJ := getEarliestDate(inbox[j].Dates)
			return startI.Before(startJ)
		})
	} else {
		// Default: longest_waiting (CreatedAt ascending)
		sort.Slice(inbox, func(i, j int) bool {
			return inbox[i].CreatedAt.Before(inbox[j].CreatedAt)
		})
	}

	// Sort Upcoming (always soonest first)
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].AcceptedDate.Start.Before(upcoming[j].AcceptedDate.Start)
	})

	render(w, r, "admin.template", AdminPageData{
		InboxEvents:    inbox,
		UpcomingEvents: upcoming,
		SortMode:       sortMode,
	})
}

func getEarliestDate(dates []db.EventDate) time.Time {
	if len(dates) == 0 {
		return time.Time{} // Should not happen for valid events
	}
	min := dates[0].Start
	for _, d := range dates {
		if d.Start.Before(min) {
			min = d.Start
		}
	}
	return min
}
