package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---- Data types ----

type Agency struct {
	ID       string
	Name     string
	URL      string
	Timezone string
	Phone    string
	FareURL  string
	Lang     string
}

type Route struct {
	ID        string
	AgencyID  string
	ShortName string
	LongName  string
	Desc      string
	Type      int
	Color     string
	TextColor string
	URL       string
	SortOrder int
}

type Trip struct {
	ID          string
	RouteID     string
	ServiceID   string
	Headsign    string
	DirectionID int // 0 or 1
}

// GTFSData holds all parsed GTFS data and pre-built visualization data.
type GTFSData struct {
	Agencies    []Agency
	Routes      []Route
	RouteVizs   []RouteViz
	FetchedAt   time.Time
}

// RouteViz is the visualization-ready struct for a single route.
type RouteViz struct {
	RouteID         string
	ShortName       string
	LongName        string
	Desc            string
	Color           string // CSS hex color (no #)
	TextColor       string // CSS hex color (no #)
	RouteType       int
	DirectionLabels [2]string
	HourRows        []HourRow
	Totals          [2]int
}

// HourRow represents one hour of service for both directions.
type HourRow struct {
	Hour        int    // raw GTFS hour (can be > 23 for overnight)
	HourDisplay string // display string like "7", "0", "1"
	Dir0Squares []struct{}
	Dir1Squares []struct{}
	Dir0Extra   int
	Dir1Extra   int
	Dir0Count   int
	Dir1Count   int
}

const maxSquares = 15
const minHour = 4
const maxHour = 27 // inclusive; covers 4am to 3am next day

// ---- GTFS fetching and parsing ----

func fetchGTFS(url string) (*GTFSData, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("reading ZIP: %w", err)
	}

	agencies := parseAgencies(zr)
	routes := parseRoutes(zr)
	serviceIDs := findWeekdayServiceIDs(zr)
	trips, dirHeadsigns := parseTrips(zr, serviceIDs)
	hourCounts := buildHourCounts(zr, trips)

	vizs := buildRouteVizs(routes, hourCounts, dirHeadsigns)

	return &GTFSData{
		Agencies:  agencies,
		Routes:    routes,
		RouteVizs: vizs,
		FetchedAt: time.Now(),
	}, nil
}

// findFile returns a *zip.File by name (case-insensitive).
func findFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, name) {
			return f
		}
	}
	return nil
}

// readCSV reads all rows from a ZIP entry, returning header and rows.
func readCSV(zf *zip.File) (header []string, rows [][]string, err error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()

	r := csv.NewReader(rc)
	r.TrimLeadingSpace = true
	r.LazyQuotes = true

	header, err = r.Read()
	if err != nil {
		return nil, nil, err
	}
	// Normalize header
	for i, h := range header {
		header[i] = strings.TrimSpace(strings.ToLower(h))
	}

	rows, err = r.ReadAll()
	return header, rows, err
}

func colIndex(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

func getField(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parseAgencies(zr *zip.Reader) []Agency {
	zf := findFile(zr, "agency.txt")
	if zf == nil {
		return nil
	}
	header, rows, err := readCSV(zf)
	if err != nil {
		log.Printf("Warning: parsing agency.txt: %v", err)
		return nil
	}
	idIdx := colIndex(header, "agency_id")
	nameIdx := colIndex(header, "agency_name")
	urlIdx := colIndex(header, "agency_url")
	tzIdx := colIndex(header, "agency_timezone")
	phoneIdx := colIndex(header, "agency_phone")
	fareIdx := colIndex(header, "agency_fare_url")
	langIdx := colIndex(header, "agency_lang")

	var agencies []Agency
	for _, row := range rows {
		agencies = append(agencies, Agency{
			ID:       getField(row, idIdx),
			Name:     getField(row, nameIdx),
			URL:      getField(row, urlIdx),
			Timezone: getField(row, tzIdx),
			Phone:    getField(row, phoneIdx),
			FareURL:  getField(row, fareIdx),
			Lang:     getField(row, langIdx),
		})
	}
	return agencies
}

func parseRoutes(zr *zip.Reader) []Route {
	zf := findFile(zr, "routes.txt")
	if zf == nil {
		return nil
	}
	header, rows, err := readCSV(zf)
	if err != nil {
		log.Printf("Warning: parsing routes.txt: %v", err)
		return nil
	}
	idIdx := colIndex(header, "route_id")
	agencyIdx := colIndex(header, "agency_id")
	shortIdx := colIndex(header, "route_short_name")
	longIdx := colIndex(header, "route_long_name")
	descIdx := colIndex(header, "route_desc")
	typeIdx := colIndex(header, "route_type")
	colorIdx := colIndex(header, "route_color")
	textIdx := colIndex(header, "route_text_color")
	urlIdx := colIndex(header, "route_url")
	sortIdx := colIndex(header, "route_sort_order")

	var routes []Route
	for _, row := range rows {
		rtype, _ := strconv.Atoi(getField(row, typeIdx))
		sortOrder, _ := strconv.Atoi(getField(row, sortIdx))
		color := strings.ToUpper(strings.TrimSpace(getField(row, colorIdx)))
		textColor := strings.ToUpper(strings.TrimSpace(getField(row, textIdx)))

		routes = append(routes, Route{
			ID:        getField(row, idIdx),
			AgencyID:  getField(row, agencyIdx),
			ShortName: getField(row, shortIdx),
			LongName:  getField(row, longIdx),
			Desc:      getField(row, descIdx),
			Type:      rtype,
			Color:     color,
			TextColor: textColor,
			URL:       getField(row, urlIdx),
			SortOrder: sortOrder,
		})
	}

	// Sort routes by sort_order if available, else short name
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].SortOrder != routes[j].SortOrder {
			return routes[i].SortOrder < routes[j].SortOrder
		}
		// Numeric-aware sort on short name
		ni, oki := strconv.Atoi(routes[i].ShortName)
		nj, okj := strconv.Atoi(routes[j].ShortName)
		if oki == nil && okj == nil {
			return ni < nj
		}
		return routes[i].ShortName < routes[j].ShortName
	})

	return routes
}

// findWeekdayServiceIDs returns a set of service_ids that operate on weekdays.
// If calendar.txt is unavailable, returns nil (all service IDs allowed).
func findWeekdayServiceIDs(zr *zip.Reader) map[string]bool {
	zf := findFile(zr, "calendar.txt")
	if zf == nil {
		log.Println("No calendar.txt found; using all service IDs")
		return nil
	}
	header, rows, err := readCSV(zf)
	if err != nil {
		log.Printf("Warning: parsing calendar.txt: %v", err)
		return nil
	}
	idIdx := colIndex(header, "service_id")
	monIdx := colIndex(header, "monday")

	if idIdx < 0 {
		return nil
	}

	ids := make(map[string]bool)
	for _, row := range rows {
		id := getField(row, idIdx)
		if monIdx < 0 || getField(row, monIdx) == "1" {
			ids[id] = true
		}
	}
	if len(ids) == 0 {
		// Fallback: accept all
		return nil
	}
	return ids
}

// parseTrips returns a map of tripID -> Trip filtered to serviceIDs,
// and a map of routeID -> directionID -> most common headsign.
func parseTrips(zr *zip.Reader, serviceIDs map[string]bool) (map[string]Trip, map[string]map[int]map[string]int) {
	zf := findFile(zr, "trips.txt")
	if zf == nil {
		return nil, nil
	}
	header, rows, err := readCSV(zf)
	if err != nil {
		log.Printf("Warning: parsing trips.txt: %v", err)
		return nil, nil
	}
	idIdx := colIndex(header, "trip_id")
	routeIdx := colIndex(header, "route_id")
	svcIdx := colIndex(header, "service_id")
	headIdx := colIndex(header, "trip_headsign")
	dirIdx := colIndex(header, "direction_id")

	trips := make(map[string]Trip)
	// routeID -> directionID -> headsign -> count
	headCounts := make(map[string]map[int]map[string]int)

	for _, row := range rows {
		svcID := getField(row, svcIdx)
		if serviceIDs != nil && !serviceIDs[svcID] {
			continue
		}
		tripID := getField(row, idIdx)
		routeID := getField(row, routeIdx)
		headsign := getField(row, headIdx)
		dirStr := getField(row, dirIdx)
		dir := 0
		if dirStr == "1" {
			dir = 1
		}
		trips[tripID] = Trip{
			ID:          tripID,
			RouteID:     routeID,
			ServiceID:   svcID,
			Headsign:    headsign,
			DirectionID: dir,
		}
		if _, ok := headCounts[routeID]; !ok {
			headCounts[routeID] = map[int]map[string]int{}
		}
		if _, ok := headCounts[routeID][dir]; !ok {
			headCounts[routeID][dir] = map[string]int{}
		}
		if headsign != "" {
			headCounts[routeID][dir][headsign]++
		}
	}
	return trips, headCounts
}

// parseHourFromGTFS parses "HH:MM:SS" and returns hour (can be > 23).
func parseHourFromGTFS(s string) (int, bool) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return 0, false
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, false
	}
	return h, true
}

// buildHourCounts reads stop_times.txt and counts departures per route/direction/hour.
// Only the first stop of each trip is counted to avoid double-counting.
// Returns: routeID -> directionID -> hour -> count
func buildHourCounts(zr *zip.Reader, trips map[string]Trip) map[string][2]map[int]int {
	zf := findFile(zr, "stop_times.txt")
	if zf == nil {
		return nil
	}
	rc, err := zf.Open()
	if err != nil {
		log.Printf("Warning: opening stop_times.txt: %v", err)
		return nil
	}
	defer rc.Close()

	r := csv.NewReader(rc)
	r.TrimLeadingSpace = true
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil
	}
	for i, h := range header {
		header[i] = strings.TrimSpace(strings.ToLower(h))
	}
	tidIdx := colIndex(header, "trip_id")
	depIdx := colIndex(header, "departure_time")
	seqIdx := colIndex(header, "stop_sequence")

	if tidIdx < 0 {
		return nil
	}

	// Track min stop_sequence seen per trip to identify first stop
	type tripEntry struct {
		minSeq int
		hour   int
	}
	firstStop := make(map[string]tripEntry) // tripID -> first stop info

	for {
		row, err := r.Read()
		if err != nil {
			break
		}
		tripID := getField(row, tidIdx)
		if _, ok := trips[tripID]; !ok {
			continue
		}
		seqStr := getField(row, seqIdx)
		seq, _ := strconv.Atoi(seqStr)

		depStr := getField(row, depIdx)
		if depStr == "" {
			// Fall back to arrival_time
			arrIdx := colIndex(header, "arrival_time")
			depStr = getField(row, arrIdx)
		}
		hour, ok := parseHourFromGTFS(depStr)
		if !ok {
			continue
		}

		if existing, seen := firstStop[tripID]; !seen || seq < existing.minSeq {
			firstStop[tripID] = tripEntry{minSeq: seq, hour: hour}
		}
	}

	// Build counts: routeID -> [2]map[int]int
	result := make(map[string][2]map[int]int)
	for tripID, entry := range firstStop {
		trip, ok := trips[tripID]
		if !ok {
			continue
		}
		if _, ok := result[trip.RouteID]; !ok {
			result[trip.RouteID] = [2]map[int]int{
				make(map[int]int),
				make(map[int]int),
			}
		}
		result[trip.RouteID][trip.DirectionID][entry.hour]++
	}
	return result
}

// mostCommonHeadsign returns the headsign with the highest count, or fallback.
func mostCommonHeadsign(counts map[string]int, fallback string) string {
	best := ""
	bestCount := 0
	for h, c := range counts {
		if c > bestCount {
			bestCount = c
			best = h
		}
	}
	if best == "" {
		return fallback
	}
	return best
}

// hourDisplay converts a raw GTFS hour to a display string.
func hourDisplay(h int) string {
	d := h % 24
	return strconv.Itoa(d)
}

// defaultRouteColors cycles through a palette when no color is in GTFS.
var defaultColors = []string{
	"0073CF", "E87722", "009A44", "C8102E", "7B2D8B",
	"00A9CE", "F4A100", "6B2737", "00685E", "C5003E",
	"8A2BE2", "2E8B57",
}

func routeColor(r Route, idx int) (bg, fg string) {
	bg = r.Color
	fg = r.TextColor
	if bg == "" {
		bg = defaultColors[idx%len(defaultColors)]
	}
	if fg == "" {
		// Determine contrasting color
		if isLightColor(bg) {
			fg = "000000"
		} else {
			fg = "FFFFFF"
		}
	}
	return bg, fg
}

func isLightColor(hex string) bool {
	if len(hex) != 6 {
		return false
	}
	r, _ := strconv.ParseInt(hex[0:2], 16, 32)
	g, _ := strconv.ParseInt(hex[2:4], 16, 32)
	b, _ := strconv.ParseInt(hex[4:6], 16, 32)
	// Relative luminance approximation
	return (r*299+g*587+b*114)/1000 > 128
}

func buildRouteVizs(routes []Route, hourCounts map[string][2]map[int]int, dirHeadsigns map[string]map[int]map[string]int) []RouteViz {
	var vizs []RouteViz
	for idx, route := range routes {
		counts, ok := hourCounts[route.ID]
		if !ok {
			continue // no trips for this route
		}

		bg, fg := routeColor(route, idx)

		dirLabels := [2]string{"Outbound", "Inbound"}
		if dm, ok := dirHeadsigns[route.ID]; ok {
			if hc, ok := dm[0]; ok {
				dirLabels[0] = mostCommonHeadsign(hc, "Outbound")
			}
			if hc, ok := dm[1]; ok {
				dirLabels[1] = mostCommonHeadsign(hc, "Inbound")
			}
		}

		var hourRows []HourRow
		totals := [2]int{0, 0}

		for h := minHour; h <= maxHour; h++ {
			c0 := counts[0][h]
			c1 := counts[1][h]
			if c0 == 0 && c1 == 0 {
				continue
			}
			totals[0] += c0
			totals[1] += c1

			sq0 := c0
			extra0 := 0
			if sq0 > maxSquares {
				extra0 = sq0 - maxSquares
				sq0 = maxSquares
			}
			sq1 := c1
			extra1 := 0
			if sq1 > maxSquares {
				extra1 = sq1 - maxSquares
				sq1 = maxSquares
			}

			hourRows = append(hourRows, HourRow{
				Hour:        h,
				HourDisplay: hourDisplay(h),
				Dir0Squares: make([]struct{}, sq0),
				Dir1Squares: make([]struct{}, sq1),
				Dir0Extra:   extra0,
				Dir1Extra:   extra1,
				Dir0Count:   c0,
				Dir1Count:   c1,
			})
		}

		if len(hourRows) == 0 {
			continue
		}

		displayName := route.ShortName
		if displayName == "" {
			displayName = route.ID
		}

		vizs = append(vizs, RouteViz{
			RouteID:         route.ID,
			ShortName:       displayName,
			LongName:        route.LongName,
			Desc:            route.Desc,
			Color:           bg,
			TextColor:       fg,
			RouteType:       route.Type,
			DirectionLabels: dirLabels,
			HourRows:        hourRows,
			Totals:          totals,
		})
	}
	return vizs
}
