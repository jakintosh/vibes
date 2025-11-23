package app

import (
	"log"
	"net/http"
	"time"

	db "event-scheduler/internal/db"

	"github.com/google/uuid"
)

func HandleGetRequest(w http.ResponseWriter, r *http.Request) {
	render(w, r, "request.template", nil)
}

func HandlePostRequest(w http.ResponseWriter, r *http.Request) {
	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Extract data
	// TODO: robust validation
	e := db.Event{
		ID:           uuid.New().String(),
		Title:        r.FormValue("title"),
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
