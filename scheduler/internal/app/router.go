package app

import (
	"net/http"
)

func BuildRouter() *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/calendar", http.StatusSeeOther)
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("GET /calendar", HandleGetCalendar)
	mux.HandleFunc("GET /request", HandleGetRequest)
	mux.HandleFunc("POST /request", HandlePostRequest)
	mux.HandleFunc("GET /admin", HandleGetAdmin)
	mux.HandleFunc("POST /event/accept/{id}", HandlePostEventAccept)
	mux.HandleFunc("GET /event/{id}", HandleGetEvent)

	return mux
}
