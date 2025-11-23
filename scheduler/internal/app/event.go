package app

import (
	"database/sql"
	"net/http"
	"time"

	db "event-scheduler/internal/db"
)

func HandleGetEvent(w http.ResponseWriter, r *http.Request) {
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

func HandlePostEventAccept(w http.ResponseWriter, r *http.Request) {
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
