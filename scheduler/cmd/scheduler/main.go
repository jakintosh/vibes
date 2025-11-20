package main

import (
	"database/sql"
	"log"
	"net/http"
	"text/template"
	"time"

	db "event-scheduler/internal/db"

	"github.com/google/uuid"
)

func main() {
	if err := db.InitDB("events.db"); err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/calendar", http.StatusSeeOther)
			return
		}
		http.NotFound(w, r)
	})

	http.HandleFunc("GET /calendar", handleCalendar)
	http.HandleFunc("GET /request", handleRequestForm)
	http.HandleFunc("POST /request", handleRequestSubmit)
	http.HandleFunc("GET /admin", handleAdmin)
	http.HandleFunc("POST /admin/accept/{id}", handleAcceptEvent)
	http.HandleFunc("GET /event/{id}", handleEventDetail)

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

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

func handleCalendar(w http.ResponseWriter, r *http.Request) {
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

func handleRequestForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, "request.template", nil)
}

func handleRequestSubmit(w http.ResponseWriter, r *http.Request) {
	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Extract data
	// TODO: robust validation
	e := db.Event{
		ID:           uuid.New().String(),
		ContactName:  r.FormValue("name"),
		ContactPhone: r.FormValue("phone"),
		ContactEmail: r.FormValue("email"),
		Description:  r.FormValue("description"),
		NeedsAV:      r.FormValue("av") == "on",
		Status:       db.StatusRequested,
	}

	// Parse dates
	// Assuming format "2006-01-02T15:04" from datetime-local input
	layout := "2006-01-02T15:04"

	// Helper to parse start/end pair
	parseDate := func(prefix string) *db.EventDate {
		startStr := r.FormValue(prefix + "_start")
		endStr := r.FormValue(prefix + "_end")
		if startStr == "" || endStr == "" {
			return nil
		}
		start, _ := time.Parse(layout, startStr)
		end, _ := time.Parse(layout, endStr)
		return &db.EventDate{Start: start, End: end}
	}

	if d := parseDate("date1"); d != nil {
		e.Dates = append(e.Dates, *d)
	}
	if d := parseDate("date2"); d != nil {
		e.Dates = append(e.Dates, *d)
	}
	if d := parseDate("date3"); d != nil {
		e.Dates = append(e.Dates, *d)
	}

	if err := db.CreateEvent(e); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Return success message or redirect
	// For HTMX, maybe just a success message
	w.Write([]byte("<div class='alert alert-success'>Request submitted!</div>"))
}

type AdminEventData struct {
	db.Event
	DateConflicts map[int]bool // Index of requested date -> isConflict
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	events, err := db.GetEvents()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Identify Accepted Events
	var acceptedEvents []db.EventDate
	for _, e := range events {
		if e.Status == db.StatusAccepted && e.AcceptedDate != nil {
			acceptedEvents = append(acceptedEvents, *e.AcceptedDate)
		}
	}

	// Enrich events with conflict data
	var adminEvents []AdminEventData
	for _, e := range events {
		data := AdminEventData{
			Event:         e,
			DateConflicts: make(map[int]bool),
		}

		if e.Status == db.StatusRequested {
			for i, d := range e.Dates {
				for _, accepted := range acceptedEvents {
					if d.Start.Before(accepted.End) && d.End.After(accepted.Start) {
						data.DateConflicts[i] = true
						break
					}
				}
			}
		}
		adminEvents = append(adminEvents, data)
	}

	render(w, r, "admin.template", map[string]any{"Events": adminEvents})
}

func handleAcceptEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	layout := "2006-01-02T15:04"
	start, _ := time.Parse(layout, r.FormValue("start"))
	end, _ := time.Parse(layout, r.FormValue("end"))
	newDate := db.EventDate{Start: start, End: end}

	// Check for conflicts with existing accepted events
	allEvents, err := db.GetEvents()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	for _, e := range allEvents {
		if e.Status == db.StatusAccepted && e.AcceptedDate != nil {
			// Check overlap
			if start.Before(e.AcceptedDate.End) && end.After(e.AcceptedDate.Start) {
				http.Error(w, "Conflict detected! Cannot accept this event.", http.StatusConflict)
				return
			}
		}
	}

	err = db.AcceptEvent(id, newDate)
	if err != nil {
		http.Error(w, "Error accepting event", http.StatusInternalServerError)
		return
	}

	// Return updated row or list
	w.Header().Set("HX-Refresh", "true") // Simple refresh for now
}

func handleEventDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	event, err := db.GetEvent(id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	render(w, r, "event_detail.template", event)
}

func render(w http.ResponseWriter, r *http.Request, tmplName string, data any) {
	// Check for HTMX header
	isHX := r.Header.Get("HX-Request") == "true"

	files := []string{"templates/" + tmplName}
	if !isHX {
		files = append(files, "templates/layout.template")
	}

	funcMap := template.FuncMap{
		"seq": func(start, end int) []int {
			var s []int
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
		"formatDate": func(t time.Time, layout string) string {
			return t.Format(layout)
		},
	}

	// Create a base template with functions
	// We use the name of the first file as the base name, which is standard for ParseFiles
	baseName := "layout.template"
	if isHX {
		baseName = tmplName
	}

	tmpl := template.New(baseName).Funcs(funcMap)
	var err error
	tmpl, err = tmpl.ParseFiles(files...)
	if err != nil {
		http.Error(w, "Template Parse Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isHX {
		err = tmpl.ExecuteTemplate(w, "content", data)
	} else {
		err = tmpl.ExecuteTemplate(w, "layout", data)
	}

	if err != nil {
		log.Println("Template execution error:", err)
	}
}
