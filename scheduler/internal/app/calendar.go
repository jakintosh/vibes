package app

import (
	"net/http"
	"time"

	db "event-scheduler/internal/db"
)

// Calendar Helpers

type CalendarViewData struct {
	ViewMode    string
	CurrentDate time.Time
	PrevDate    string
	NextDate    string
	MonthName   string
	WeekStart   time.Time
	WeekEnd     time.Time
	GridDays    []time.Time // For month view
	Events      []DisplayEvent
}

type DisplayEvent struct {
	Event       db.Event
	DisplayDate time.Time // The specific day this segment belongs to
	Start       time.Time // Clipped start for this day
	End         time.Time // Clipped end for this day
	IsConflict  bool
	Top         float64 // For week view positioning (0-100%)
	Height      float64 // For week view positioning (0-100%)
}

func HandleGetCalendar(w http.ResponseWriter, r *http.Request) {
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "month"
	}

	dateStr := r.URL.Query().Get("date")
	currentDate := time.Now()
	if dateStr != "" {
		if d, err := time.Parse("2006-01-02", dateStr); err == nil {
			currentDate = d
		}
	}

	data := CalendarViewData{
		ViewMode:    view,
		CurrentDate: currentDate,
		MonthName:   currentDate.Format("January 2006"),
	}

	// Calculate ranges and navigation
	var start, end time.Time
	dateFormat := "2006-01-02"

	switch view {
	case "week":
		// Start of week (Sunday)
		weekday := int(currentDate.Weekday())
		start = currentDate.AddDate(0, 0, -weekday).Truncate(24 * time.Hour)
		end = start.AddDate(0, 0, 7)
		data.WeekStart = start
		data.WeekEnd = end

		data.PrevDate = start.AddDate(0, 0, -7).Format(dateFormat)
		data.NextDate = start.AddDate(0, 0, 7).Format(dateFormat)

	case "month":
		// Start of month
		start = time.Date(currentDate.Year(), currentDate.Month(), 1, 0, 0, 0, 0, currentDate.Location())
		// Start of grid (Sunday before start of month)
		weekday := int(start.Weekday())
		gridStart := start.AddDate(0, 0, -weekday)

		// End of month
		// nextMonth := start.AddDate(0, 1, 0)
		// End of grid (Saturday after end of month)
		// We want 6 rows of 7 days = 42 days to be safe, or just enough to cover
		end = gridStart.AddDate(0, 0, 42)

		// Populate GridDays
		for d := gridStart; d.Before(end); d = d.AddDate(0, 0, 1) {
			data.GridDays = append(data.GridDays, d)
		}

		data.PrevDate = start.AddDate(0, -1, 0).Format(dateFormat)
		data.NextDate = start.AddDate(0, 1, 0).Format(dateFormat)

	case "agenda":
		// Just show everything or a large range
		start = time.Now().AddDate(-1, 0, 0)
		end = time.Now().AddDate(1, 0, 0)
		// Agenda navigation could be month-based or just infinite scroll, keeping simple for now
		data.PrevDate = currentDate.AddDate(0, -1, 0).Format(dateFormat)
		data.NextDate = currentDate.AddDate(0, 1, 0).Format(dateFormat)
	}

	allEvents, err := db.GetEvents()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Process Events
	var displayEvents []DisplayEvent

	// 1. Identify Accepted Events for Conflict Checking
	var acceptedEvents []db.EventDate
	for _, e := range allEvents {
		if e.Status == db.StatusAccepted && e.AcceptedDate != nil {
			acceptedEvents = append(acceptedEvents, *e.AcceptedDate)
		}
	}

	for _, e := range allEvents {
		// Determine which dates to process
		var datesToCheck []db.EventDate
		if e.Status == db.StatusAccepted && e.AcceptedDate != nil {
			datesToCheck = append(datesToCheck, *e.AcceptedDate)
		} else if e.Status == db.StatusRequested {
			datesToCheck = e.Dates
		}

		for _, d := range datesToCheck {
			// Check overlap with view range
			if d.End.Before(start) || d.Start.After(end) {
				continue
			}

			// Check Conflict (only for requested events)
			isConflict := false
			if e.Status == db.StatusRequested {
				for _, accepted := range acceptedEvents {
					if d.Start.Before(accepted.End) && d.End.After(accepted.Start) {
						isConflict = true
						break
					}
				}
			}

			// Split Multi-day Events
			curr := d.Start
			for curr.Before(d.End) {
				dayEnd := time.Date(curr.Year(), curr.Month(), curr.Day(), 23, 59, 59, 999999999, curr.Location())

				segmentEnd := d.End
				if segmentEnd.After(dayEnd) {
					segmentEnd = dayEnd
				}

				// Calculate positioning for Week View
				// Day start is 0:00, End is 24:00
				dayStart := time.Date(curr.Year(), curr.Month(), curr.Day(), 0, 0, 0, 0, curr.Location())
				totalMinutes := 24 * 60.0
				startMinutes := curr.Sub(dayStart).Minutes()
				durationMinutes := segmentEnd.Sub(curr).Minutes()

				top := (startMinutes / totalMinutes) * 100
				height := (durationMinutes / totalMinutes) * 100

				displayEvents = append(displayEvents, DisplayEvent{
					Event:       e,
					DisplayDate: dayStart,
					Start:       curr,
					End:         segmentEnd,
					IsConflict:  isConflict,
					Top:         top,
					Height:      height,
				})

				curr = dayEnd.Add(1 * time.Nanosecond) // Next day start
			}
		}
	}

	data.Events = displayEvents

	render(w, r, "calendar.template", data)
}
