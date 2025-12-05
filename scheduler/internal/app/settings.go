package app

import (
	"net/http"
)

func HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	render(w, r, "settings.template", nil)
}
