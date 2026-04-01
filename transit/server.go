package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed templates/index.html
var templateFS embed.FS

type PageData struct {
	Agencies    []Agency
	RouteVizs   []RouteViz
	Feed        *FeedInfo
	FeedURL     string
	FetchedAt   string
	HasCompare  bool
	CompareFeed *FeedInfo
	CompareURL  string
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

func buildTemplate() (*template.Template, error) {
	funcMap := template.FuncMap{
		"routeTypeName": routeTypeName,
		"formatDate": func(t time.Time) string {
			return t.Format("Jan 2006")
		},
		"formatDateFull": func(t time.Time) string {
			return t.Format("Jan 2, 2006")
		},
	}
	return template.New("index.html").Funcs(funcMap).ParseFS(templateFS, "templates/index.html")
}

func buildPageData(data *GTFSData, feedURL string, compareData *GTFSData, compareURL string) PageData {
	pd := PageData{
		Agencies:   data.Agencies,
		RouteVizs:  data.RouteVizs,
		Feed:       data.Feed,
		FeedURL:    feedURL,
		FetchedAt:  data.FetchedAt.Format(time.RFC1123),
		HasCompare: compareData != nil,
		CompareURL: compareURL,
	}
	if compareData != nil {
		pd.CompareFeed = compareData.Feed
	}
	return pd
}

func renderToFile(data *GTFSData, feedURL string, compareData *GTFSData, compareURL string, path string) error {
	tmpl, err := buildTemplate()
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()
	if err := tmpl.Execute(f, buildPageData(data, feedURL, compareData, compareURL)); err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}
	return nil
}

func startServer(data *GTFSData, feedURL string, compareData *GTFSData, compareURL string, port int) error {
	tmpl, err := buildTemplate()
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	pageData := buildPageData(data, feedURL, compareData, compareURL)

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
