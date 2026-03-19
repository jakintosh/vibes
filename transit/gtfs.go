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

// dayColumns maps day index (Monday=0 … Sunday=6) to the GTFS calendar.txt column name.
var dayColumns = [7]string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

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

// FeedInfo holds feed-level metadata from feed_info.txt.
type FeedInfo struct {
	PublisherName string
	Version       string
	StartDate     time.Time
	EndDate       time.Time
	HasDates      bool
}

// GTFSData holds all parsed GTFS data and pre-built visualization data.
type GTFSData struct {
	Agencies  []Agency
	Routes    []Route
	RouteVizs []RouteViz
	FetchedAt time.Time
	Feed      *FeedInfo
}

// DayService holds pre-built hour rows and totals for one day of the week.
type DayService struct {
	DayKey   string // "monday", "tuesday", …, "sunday"
	HourRows []HourRow
	Totals   [2]int
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
	DayServices     []DayService // one entry per day that has service
	EffectiveFrom   time.Time
	EffectiveTo     time.Time
	HasDates        bool
	IsActive        bool // today is within the effective date range
	ExpiringSoon    bool // active but expiring within 30 days
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
	Empty       bool // no trips in either direction this hour
}

const maxSquares = 15
const minHour = 4
const maxHour = 27 // inclusive; covers 4am to 3am next day

// ---- GTFS fetching and parsing ----

// parseGTFSDate parses GTFS YYYYMMDD date format.
func parseGTFSDate(s string) (time.Time, bool) {
	t, err := time.Parse("20060102", strings.TrimSpace(s))
	return t, err == nil
}

// parseFeedInfo reads feed_info.txt and returns feed-level metadata.
func parseFeedInfo(zr *zip.Reader) *FeedInfo {
	zf := findFile(zr, "feed_info.txt")
	if zf == nil {
		return nil
	}
	header, rows, err := readCSV(zf)
	if err != nil || len(rows) == 0 {
		return nil
	}
	row := rows[0]
	fi := &FeedInfo{
		PublisherName: getField(row, colIndex(header, "feed_publisher_name")),
		Version:       getField(row, colIndex(header, "feed_version")),
	}
	if t, ok := parseGTFSDate(getField(row, colIndex(header, "feed_start_date"))); ok {
		fi.StartDate = t
		fi.HasDates = true
	}
	if t, ok := parseGTFSDate(getField(row, colIndex(header, "feed_end_date"))); ok {
		fi.EndDate = t
	}
	return fi
}

// parseCalendarDateRanges returns serviceID -> [start, end] from calendar.txt.
func parseCalendarDateRanges(zr *zip.Reader) map[string][2]time.Time {
	zf := findFile(zr, "calendar.txt")
	if zf == nil {
		return nil
	}
	header, rows, err := readCSV(zf)
	if err != nil {
		return nil
	}
	idIdx := colIndex(header, "service_id")
	startIdx := colIndex(header, "start_date")
	endIdx := colIndex(header, "end_date")
	if idIdx < 0 || startIdx < 0 || endIdx < 0 {
		return nil
	}
	result := make(map[string][2]time.Time)
	for _, row := range rows {
		id := getField(row, idIdx)
		start, okS := parseGTFSDate(getField(row, startIdx))
		end, okE := parseGTFSDate(getField(row, endIdx))
		if okS && okE {
			result[id] = [2]time.Time{start, end}
		}
	}
	return result
}

// buildRouteServiceIDs maps each routeID to the set of serviceIDs used by its trips.
func buildRouteServiceIDs(trips map[string]Trip) map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	for _, trip := range trips {
		if result[trip.RouteID] == nil {
			result[trip.RouteID] = make(map[string]bool)
		}
		result[trip.RouteID][trip.ServiceID] = true
	}
	return result
}

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
	feedInfo := parseFeedInfo(zr)
	calendarRanges := parseCalendarDateRanges(zr)

	// Parse all trips once (no day filter) to get direction labels and route service IDs.
	allTrips, dirHeadsigns := parseTrips(zr, nil)
	routeServiceIDs := buildRouteServiceIDs(allTrips)

	// Build per-day service ID sets, then filter allTrips in memory (no re-read of trips.txt).
	var dayTripMaps [7]map[string]Trip
	for d, col := range dayColumns {
		ids := findServiceIDsWhere(zr, col)
		if len(ids) == 0 {
			continue
		}
		dt := make(map[string]Trip)
		for tripID, trip := range allTrips {
			if ids[trip.ServiceID] {
				dt[tripID] = trip
			}
		}
		if len(dt) > 0 {
			dayTripMaps[d] = dt
		}
	}

	// Build hour counts for all 7 days in a single pass over stop_times.txt.
	dayCounts := buildAllDayHourCounts(zr, dayTripMaps)

	vizs := buildRouteVizs(routes, dayCounts, dirHeadsigns, calendarRanges, routeServiceIDs)

	return &GTFSData{
		Agencies:  agencies,
		Routes:    routes,
		RouteVizs: vizs,
		FetchedAt: time.Now(),
		Feed:      feedInfo,
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

	// Sort routes numerically by short name (1, 2, 5, 10, 50, 100 …).
	// Non-numeric names sort alphabetically after all numeric names.
	sort.SliceStable(routes, func(i, j int) bool {
		ni, oki := strconv.Atoi(routes[i].ShortName)
		nj, okj := strconv.Atoi(routes[j].ShortName)
		switch {
		case oki == nil && okj == nil:
			return ni < nj
		case oki == nil:
			return true // numeric before alpha
		case okj == nil:
			return false
		default:
			return routes[i].ShortName < routes[j].ShortName
		}
	})

	return routes
}

// findServiceIDsWhere returns service IDs from calendar.txt where any of the given
// day columns equal "1". Returns nil if no matches or calendar.txt is missing.
func findServiceIDsWhere(zr *zip.Reader, days ...string) map[string]bool {
	zf := findFile(zr, "calendar.txt")
	if zf == nil {
		return nil
	}
	header, rows, err := readCSV(zf)
	if err != nil {
		log.Printf("Warning: parsing calendar.txt: %v", err)
		return nil
	}
	idIdx := colIndex(header, "service_id")
	if idIdx < 0 {
		return nil
	}
	dayIdxs := make([]int, len(days))
	for i, d := range days {
		dayIdxs[i] = colIndex(header, d)
	}
	ids := make(map[string]bool)
	for _, row := range rows {
		id := getField(row, idIdx)
		for _, di := range dayIdxs {
			if di >= 0 && getField(row, di) == "1" {
				ids[id] = true
				break
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// parseTrips returns a map of tripID -> Trip filtered to serviceIDs (nil = all),
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

// buildAllDayHourCounts reads stop_times.txt once and counts departures per
// route/direction/hour for each of the 7 days. dayTrips[d] is the trip map for day d
// (nil means no service that day). Returns counts indexed by day (Monday=0…Sunday=6).
func buildAllDayHourCounts(zr *zip.Reader, dayTrips [7]map[string]Trip) [7]map[string][2]map[int]int {
	var results [7]map[string][2]map[int]int
	for i := range results {
		results[i] = make(map[string][2]map[int]int)
	}

	zf := findFile(zr, "stop_times.txt")
	if zf == nil {
		return results
	}
	rc, err := zf.Open()
	if err != nil {
		log.Printf("Warning: opening stop_times.txt: %v", err)
		return results
	}
	defer rc.Close()

	r := csv.NewReader(rc)
	r.TrimLeadingSpace = true
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return results
	}
	for i, h := range header {
		header[i] = strings.TrimSpace(strings.ToLower(h))
	}
	tidIdx := colIndex(header, "trip_id")
	depIdx := colIndex(header, "departure_time")
	seqIdx := colIndex(header, "stop_sequence")
	arrIdx := colIndex(header, "arrival_time")

	if tidIdx < 0 {
		return results
	}

	// Build union of all trip IDs needed across all days.
	needed := make(map[string]bool)
	for _, dt := range dayTrips {
		for tripID := range dt {
			needed[tripID] = true
		}
	}

	// Track minimum stop_sequence and its hour per trip.
	type tripEntry struct {
		minSeq int
		hour   int
	}
	firstStop := make(map[string]tripEntry)

	for {
		row, err := r.Read()
		if err != nil {
			break
		}
		tripID := getField(row, tidIdx)
		if !needed[tripID] {
			continue
		}
		seqStr := getField(row, seqIdx)
		seq, _ := strconv.Atoi(seqStr)

		depStr := getField(row, depIdx)
		if depStr == "" {
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

	// Distribute first-stop hours into per-day counts.
	for tripID, entry := range firstStop {
		for d, dt := range dayTrips {
			if dt == nil {
				continue
			}
			trip, ok := dt[tripID]
			if !ok {
				continue
			}
			routeID := trip.RouteID
			if results[d][routeID][0] == nil {
				results[d][routeID] = [2]map[int]int{
					make(map[int]int),
					make(map[int]int),
				}
			}
			results[d][routeID][trip.DirectionID][entry.hour]++
		}
	}

	return results
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

// buildHourRows converts per-direction hour counts into HourRow slice and totals.
func buildHourRows(counts [2]map[int]int) (rows []HourRow, totals [2]int, hasTrips bool) {
	for h := minHour; h <= maxHour; h++ {
		c0 := counts[0][h]
		c1 := counts[1][h]
		empty := c0 == 0 && c1 == 0
		if !empty {
			hasTrips = true
			totals[0] += c0
			totals[1] += c1
		}
		sq0, extra0 := c0, 0
		if sq0 > maxSquares {
			extra0 = sq0 - maxSquares
			sq0 = maxSquares
		}
		sq1, extra1 := c1, 0
		if sq1 > maxSquares {
			extra1 = sq1 - maxSquares
			sq1 = maxSquares
		}
		rows = append(rows, HourRow{
			Hour:        h,
			HourDisplay: hourDisplay(h),
			Dir0Squares: make([]struct{}, sq0),
			Dir1Squares: make([]struct{}, sq1),
			Dir0Extra:   extra0,
			Dir1Extra:   extra1,
			Dir0Count:   c0,
			Dir1Count:   c1,
			Empty:       empty,
		})
	}
	return
}

func buildRouteVizs(routes []Route, dayCounts [7]map[string][2]map[int]int, dirHeadsigns map[string]map[int]map[string]int, calendarRanges map[string][2]time.Time, routeServiceIDs map[string]map[string]bool) []RouteViz {
	now := time.Now().Truncate(24 * time.Hour)
	var vizs []RouteViz
	for idx, route := range routes {
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

		// Compute effective date range from calendar entries for this route's service IDs.
		var effectiveFrom, effectiveTo time.Time
		hasDates := false
		if svcIDs, ok := routeServiceIDs[route.ID]; ok {
			for svcID := range svcIDs {
				if dr, ok := calendarRanges[svcID]; ok {
					if !hasDates || dr[0].Before(effectiveFrom) {
						effectiveFrom = dr[0]
					}
					if !hasDates || dr[1].After(effectiveTo) {
						effectiveTo = dr[1]
					}
					hasDates = true
				}
			}
		}
		isActive := hasDates && !now.Before(effectiveFrom) && !now.After(effectiveTo)
		expiringSoon := isActive && effectiveTo.Sub(now) <= 30*24*time.Hour

		// Build a DayService entry for each day that has service.
		var dayServices []DayService
		for d, col := range dayColumns {
			counts, ok := dayCounts[d][route.ID]
			if !ok {
				continue
			}
			rows, totals, hasTrips := buildHourRows(counts)
			if !hasTrips {
				continue
			}
			dayServices = append(dayServices, DayService{
				DayKey:   col,
				HourRows: rows,
				Totals:   totals,
			})
		}

		if len(dayServices) == 0 {
			continue // no service on any day
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
			DayServices:     dayServices,
			EffectiveFrom:   effectiveFrom,
			EffectiveTo:     effectiveTo,
			HasDates:        hasDates,
			IsActive:        isActive,
			ExpiringSoon:    expiringSoon,
		})
	}
	return vizs
}
