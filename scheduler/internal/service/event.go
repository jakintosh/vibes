package service

import (
	"errors"
	"time"

	db "event-scheduler/internal/db"

	"github.com/google/uuid"
)

func GetEvent(id string) (*db.Event, error) {
	return db.GetEvent(id)
}

func AcceptEvent(id string, start, end time.Time) error {
	// Check for conflicts with existing accepted events
	allEvents, err := db.GetEvents()
	if err != nil {
		return err
	}

	for _, e := range allEvents {
		if e.Status == db.StatusAccepted && e.AcceptedDate != nil {
			// Check overlap
			if start.Before(e.AcceptedDate.End) && end.After(e.AcceptedDate.Start) {
				return errors.New("conflict detected")
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
