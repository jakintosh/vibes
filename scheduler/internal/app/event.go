package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
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

	proposedCost, err := strconv.ParseFloat(r.FormValue("proposed_cost"), 64)
	if err != nil {
		http.Error(w, "Invalid proposed cost", http.StatusBadRequest)
		return
	}

	depositAmount, err := strconv.ParseFloat(r.FormValue("deposit_amount"), 64)
	if err != nil && r.FormValue("deposit_amount") != "" {
		http.Error(w, "Invalid deposit amount", http.StatusBadRequest)
		return
	}

	err = service.AcceptEvent(id, start, end, proposedCost, depositAmount)
	if err != nil {
		if errors.Is(err, service.ErrConflict) {
			http.Error(w, "Conflict detected! Cannot accept this event.", http.StatusConflict)
		} else if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "Invalid event status for accepting", http.StatusBadRequest)
		} else if errors.Is(err, service.ErrInvalidPayment) {
			http.Error(w, "Invalid payment details for this event", http.StatusBadRequest)
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

func HandlePostEventPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil {
		http.Error(w, "Invalid payment amount", http.StatusBadRequest)
		return
	}
	if err := service.RecordPayment(id, amount); err != nil {
		if errors.Is(err, service.ErrInvalidPayment) {
			http.Error(w, "Invalid payment amount", http.StatusBadRequest)
			return
		}
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "Payment not allowed in current state", http.StatusBadRequest)
			return
		}
		http.Error(w, "Error recording payment", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Refresh", "true")
}

func HandlePostEventSettle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := service.SettleRefundable(id); err != nil {
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "Invalid payment status for settlement", http.StatusBadRequest)
			return
		}
		http.Error(w, "Error settling payment", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Refresh", "true")
}

func HandlePostEventStaffing(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	opener := r.FormValue("opener")
	closer := r.FormValue("closer")
	notes := r.FormValue("notes")

	if err := service.UpdateStaffing(id, opener, closer, notes); err != nil {
		if errors.Is(err, service.ErrInvalidStatusTransition) {
			http.Error(w, "Staffing can only be assigned to accepted or confirmed events", http.StatusBadRequest)
			return
		}
		http.Error(w, "Error updating staffing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Refresh", "true")
}
