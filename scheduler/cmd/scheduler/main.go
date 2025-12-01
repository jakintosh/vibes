package main

import (
	"log"
	"net/http"

	"event-scheduler/internal/app"
	db "event-scheduler/internal/db"
)

func main() {
	if err := db.InitDB("events.db"); err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	router := app.BuildRouter()

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
