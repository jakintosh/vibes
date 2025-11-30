package app

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"event-scheduler/internal/service"
)

func HandleGetEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	event, err := service.GetEvent(id)
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

	err := service.AcceptEvent(id, start, end)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			http.Error(w, "Conflict detected! Cannot accept this event.", http.StatusConflict)
		} else if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "Invalid event status for accepting", http.StatusBadRequest)
		} else {
			http.Error(w, "Error accepting event", http.StatusInternalServerError)
		}
		return
	}

	// Return updated row or list
	w.Header().Set("HX-Refresh", "true") // Simple refresh for now
}

func HandlePostEventConfirm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := service.ConfirmEvent(id); err != nil {
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "Invalid event status for confirming", http.StatusBadRequest)
			return
		}
		http.Error(w, "Error confirming event", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Refresh", "true")
}

func HandlePostEventWithdraw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := service.WithdrawEvent(id); err != nil {
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "Invalid event status for withdrawing", http.StatusBadRequest)
			return
		}
		http.Error(w, "Error withdrawing event", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Refresh", "true")
}

func HandlePostEventDeny(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := service.DenyEvent(id); err != nil {
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "Invalid event status for denying", http.StatusBadRequest)
			return
		}
		http.Error(w, "Error denying event", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Refresh", "true")
}

func HandlePostEventCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := service.CancelEvent(id); err != nil {
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "Invalid event status for canceling", http.StatusBadRequest)
			return
		}
		http.Error(w, "Error canceling event", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Refresh", "true")
}
