package service

import (
	"time"

	db "event-scheduler/internal/db"
)

type DisplayEvent struct {
	Event         db.Event
	DisplayDate   time.Time // The specific day this segment belongs to
	Start         time.Time // Clipped start for this day
	End           time.Time // Clipped end for this day
	IsConflict    bool
	NeedsStaffing bool    // For accepted/confirmed events without full staffing
	Top           float64 // For week view positioning (0-100%)
	Height        float64 // For week view positioning (0-100%)
}

type AdminEventData struct {
	db.Event
	DateConflicts map[int]bool // Index of requested date -> isConflict
}
