package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	url := flag.String("url", "", "GTFS feed URL (required)")
	compare := flag.String("compare", "", "Second (previous) GTFS feed URL to compare against")
	port := flag.Int("port", 8080, "HTTP port to listen on")
	render := flag.String("render", "", "Render HTML to this file path instead of serving")
	flag.Parse()

	if *url == "" {
		fmt.Fprintln(os.Stderr, "Error: --url is required")
		fmt.Fprintln(os.Stderr, "Usage: transit --url <gtfs-feed-url> [--compare <prev-gtfs-url>] [--port 8080] [--render out.html]")
		os.Exit(1)
	}

	log.Printf("Fetching GTFS data from %s ...", *url)
	data, err := fetchGTFS(*url)
	if err != nil {
		log.Fatalf("Failed to fetch/parse GTFS: %v", err)
	}

	agencyName := "Transit"
	if len(data.Agencies) > 0 {
		agencyName = data.Agencies[0].Name
	}
	log.Printf("Loaded %d routes for %s", len(data.RouteVizs), agencyName)

	var compareData *GTFSData
	if *compare != "" {
		log.Printf("Fetching compare GTFS data from %s ...", *compare)
		compareData, err = fetchGTFS(*compare)
		if err != nil {
			log.Fatalf("Failed to fetch/parse compare GTFS: %v", err)
		}
		log.Printf("Loaded %d compare routes; computing deltas ...", len(compareData.RouteVizs))
		applyDelta(data.RouteVizs, compareData)
	}

	if *render != "" {
		if err := renderToFile(data, *url, compareData, *compare, *render); err != nil {
			log.Fatalf("Render failed: %v", err)
		}
		log.Printf("Rendered to %s", *render)
		return
	}

	log.Printf("Starting server on http://localhost:%d", *port)
	if err := startServer(data, *url, compareData, *compare, *port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
