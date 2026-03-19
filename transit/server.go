package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"
)

//go:embed templates/index.html
var templateFS embed.FS

type PageData struct {
	Agencies   []Agency
	RouteVizs  []RouteViz
	Feed       *FeedInfo
	FeedURL    string
	FetchedAt  string
	HasWeekend bool
}

var routeTypeNames = map[int]string{
	0: "Tram",
	1: "Metro",
	2: "Rail",
	3: "Bus",
	4: "Ferry",
	5: "Cable Car",
	6: "Gondola",
	7: "Funicular",
	11: "Trolleybus",
	12: "Monorail",
}

func routeTypeName(t int) string {
	if name, ok := routeTypeNames[t]; ok {
		return name
	}
	return "Transit"
}

func startServer(data *GTFSData, feedURL string, port int) error {
	funcMap := template.FuncMap{
		"routeTypeName": routeTypeName,
		"formatDate": func(t time.Time) string {
			return t.Format("Jan 2006")
		},
		"formatDateFull": func(t time.Time) string {
			return t.Format("Jan 2, 2006")
		},
	}
	tmpl, err := template.New("index.html").Funcs(funcMap).ParseFS(templateFS, "templates/index.html")
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	pageData := PageData{
		Agencies:   data.Agencies,
		RouteVizs:  data.RouteVizs,
		Feed:       data.Feed,
		FeedURL:    feedURL,
		FetchedAt:  data.FetchedAt.Format(time.RFC1123),
		HasWeekend: data.HasWeekend,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, pageData); err != nil {
			log.Printf("Template error: %v", err)
			http.Error(w, "Internal server error", 500)
		}
	})

	addr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(addr, nil)
}
