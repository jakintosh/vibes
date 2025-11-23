package app

import (
	"net/http"
	"time"

	"event-scheduler/internal/service"
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
	Events      []service.DisplayEvent
	HighlightID string
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

	if highlight := r.URL.Query().Get("highlight"); highlight != "" {
		data.HighlightID = highlight
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

	events, err := service.GetCalendarEvents(start, end)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data.Events = events

	render(w, r, "calendar.template", data)
}
