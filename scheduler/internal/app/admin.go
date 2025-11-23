package app

import (
	"net/http"

	"event-scheduler/internal/service"
)

type AdminPageData struct {
	InboxEvents    []service.AdminEventData
	UpcomingEvents []service.AdminEventData
	SortMode       string
}

func HandleGetAdmin(w http.ResponseWriter, r *http.Request) {
	sortMode := r.URL.Query().Get("sort")
	if sortMode == "" {
		sortMode = "longest_waiting"
	}

	inbox, upcoming, err := service.GetAdminDashboardData(sortMode)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	render(w, r, "admin.template", AdminPageData{
		InboxEvents:    inbox,
		UpcomingEvents: upcoming,
		SortMode:       sortMode,
	})
}
