package service

import (
	"sort"
	"time"

	db "event-scheduler/internal/db"
)

func GetAdminDashboardData(sortMode string) (inbox []AdminEventData, upcoming []AdminEventData, needsStaffing []AdminEventData, err error) {
	events, err := db.GetEvents()
	if err != nil {
		return nil, nil, nil, err
	}

	// Identify Accepted Events for conflict checking
	var acceptedEvents []db.EventDate
	for _, e := range events {
		if isScheduledStatus(e.Status) && e.AcceptedDate != nil {
			acceptedEvents = append(acceptedEvents, *e.AcceptedDate)
		}
	}

	now := time.Now()

	for _, e := range events {
		if err := normalizePayment(&e); err != nil {
			return nil, nil, nil, err
		}

		data := AdminEventData{
			Event:         e,
			DateConflicts: make(map[int]bool),
		}

		if e.Status == db.StatusRequested {
			// Calculate conflicts
			for i, d := range e.Dates {
				for _, accepted := range acceptedEvents {
					if d.Start.Before(accepted.End) && d.End.After(accepted.Start) {
						data.DateConflicts[i] = true
						break
					}
				}
			}
			inbox = append(inbox, data)
		} else if isScheduledStatus(e.Status) && e.AcceptedDate != nil {
			// Only show future events in Upcoming
			if e.AcceptedDate.Start.After(now) {
				upcoming = append(upcoming, data)
				// Also add to needsStaffing if missing opener or closer
				if NeedsStaffing(&e) {
					needsStaffing = append(needsStaffing, data)
				}
			}
		}
	}

	// Sort Inbox
	if sortMode == "soonest" {
		// Sort by earliest requested date
		sort.Slice(inbox, func(i, j int) bool {
			startI := getEarliestDate(inbox[i].Dates)
			startJ := getEarliestDate(inbox[j].Dates)
			return startI.Before(startJ)
		})
	} else {
		// Default: longest_waiting (CreatedAt ascending)
		sort.Slice(inbox, func(i, j int) bool {
			return inbox[i].CreatedAt.Before(inbox[j].CreatedAt)
		})
	}

	// Sort Upcoming and NeedsStaffing (always soonest first)
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].AcceptedDate.Start.Before(upcoming[j].AcceptedDate.Start)
	})
	sort.Slice(needsStaffing, func(i, j int) bool {
		return needsStaffing[i].AcceptedDate.Start.Before(needsStaffing[j].AcceptedDate.Start)
	})

	return inbox, upcoming, needsStaffing, nil
}

func getEarliestDate(dates []db.EventDate) time.Time {
	if len(dates) == 0 {
		return time.Time{} // Should not happen for valid events
	}
	min := dates[0].Start
	for _, d := range dates {
		if d.Start.Before(min) {
			min = d.Start
		}
	}
	return min
}
