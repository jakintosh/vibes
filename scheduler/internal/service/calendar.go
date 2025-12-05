package service

import (
	"time"

	db "event-scheduler/internal/db"
)

func GetCalendarEvents(start, end time.Time) ([]DisplayEvent, error) {
	allEvents, err := db.GetEvents()
	if err != nil {
		return nil, err
	}

	// Process Events
	var displayEvents []DisplayEvent

	// 1. Identify Accepted Events for Conflict Checking
	var acceptedEvents []db.EventDate
	for _, e := range allEvents {
		if isScheduledStatus(e.Status) && e.AcceptedDate != nil {
			acceptedEvents = append(acceptedEvents, *e.AcceptedDate)
		}
	}

	for _, e := range allEvents {
		// Determine which dates to process
		var datesToCheck []db.EventDate
		if isScheduledStatus(e.Status) && e.AcceptedDate != nil {
			datesToCheck = append(datesToCheck, *e.AcceptedDate)
		} else if e.Status == db.StatusRequested {
			datesToCheck = e.Dates
		} else {
			continue
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
					Event:         e,
					DisplayDate:   dayStart,
					Start:         curr,
					End:           segmentEnd,
					IsConflict:    isConflict,
					NeedsStaffing: NeedsStaffing(&e),
					Top:           top,
					Height:        height,
				})

				curr = dayEnd.Add(1 * time.Nanosecond) // Next day start
			}
		}
	}

	return displayEvents, nil
}
