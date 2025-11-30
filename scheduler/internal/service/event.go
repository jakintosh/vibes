package service

import (
	"errors"
	"time"

	db "event-scheduler/internal/db"

	"github.com/google/uuid"
)

var (
	ErrConflict                = errors.New("conflict detected")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
)

func GetEvent(id string) (*db.Event, error) {
	return db.GetEvent(id)
}

func AcceptEvent(id string, start, end time.Time) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusRequested {
		return ErrInvalidStatusTransition
	}

	// Check for conflicts with existing accepted events
	allEvents, err := db.GetEvents()
	if err != nil {
		return err
	}

	for _, e := range allEvents {
		if isScheduledStatus(e.Status) && e.AcceptedDate != nil {
			// Check overlap
			if start.Before(e.AcceptedDate.End) && end.After(e.AcceptedDate.Start) {
				return ErrConflict
			}
		}
	}

	newDate := db.EventDate{Start: start, End: end}
	return db.AcceptEvent(id, newDate)
}

func CreateEvent(e db.Event) error {
	// Set system fields
	e.ID = uuid.New().String()
	e.Status = db.StatusRequested
	// CreatedAt is handled by db.CreateEvent if zero, but we can set it here too
	e.CreatedAt = time.Now()

	return db.CreateEvent(e)
}

func ConfirmEvent(id string) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusAccepted || event.AcceptedDate == nil {
		return ErrInvalidStatusTransition
	}

	return db.UpdateEventStatus(id, db.StatusConfirmed)
}

func WithdrawEvent(id string) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusAccepted {
		return ErrInvalidStatusTransition
	}

	return db.UpdateEventStatus(id, db.StatusWithdrawn)
}

func DenyEvent(id string) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusRequested {
		return ErrInvalidStatusTransition
	}

	return db.UpdateEventStatus(id, db.StatusDenied)
}

func CancelEvent(id string) error {
	event, err := db.GetEvent(id)
	if err != nil {
		return err
	}

	if event.Status != db.StatusAccepted && event.Status != db.StatusConfirmed {
		return ErrInvalidStatusTransition
	}

	return db.UpdateEventStatus(id, db.StatusCanceled)
}

func isScheduledStatus(status db.EventStatus) bool {
	return status == db.StatusAccepted || status == db.StatusConfirmed
}
