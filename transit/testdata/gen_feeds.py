#!/usr/bin/env python3
"""
Generate two minimal GTFS ZIP fixtures for testing the --compare delta view.

  previous.zip  – older schedule (less frequent on some routes)
  current.zip   – newer schedule (added trips on some routes, removed on others)

Routes:
  1  Route One   WD: +2 trips 7-9am peak vs previous (additions)
                 WD: -1 trip  17-19 pm vs previous  (removals)
  2  Route Two   WD: +1 trip per hour all day       (additions)
  3  Route Three WD: -1 trip per hour all day       (removals)
"""
import csv, io, zipfile, os

OUT = os.path.dirname(os.path.abspath(__file__))

# ── helpers ──────────────────────────────────────────────────────────────────

def csv_bytes(rows: list[dict]) -> bytes:
    buf = io.StringIO()
    if not rows:
        return b""
    w = csv.DictWriter(buf, fieldnames=list(rows[0].keys()))
    w.writeheader()
    w.writerows(rows)
    return buf.getvalue().encode()


def make_zip(files: dict[str, bytes]) -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as zf:
        for name, data in files.items():
            zf.writestr(name, data)
    return buf.getvalue()


# ── static tables (shared structure, dates differ) ───────────────────────────

AGENCY = [
    {"agency_id": "MOCK", "agency_name": "Test Transit Agency",
     "agency_url": "http://example.com", "agency_timezone": "America/New_York",
     "agency_lang": "en", "agency_phone": "555-0100"}
]

ROUTES = [
    {"route_id": "R1", "agency_id": "MOCK", "route_short_name": "1",
     "route_long_name": "Route One",   "route_type": "3",
     "route_color": "0073CF", "route_text_color": "FFFFFF"},
    {"route_id": "R2", "agency_id": "MOCK", "route_short_name": "2",
     "route_long_name": "Route Two",   "route_type": "3",
     "route_color": "E87722", "route_text_color": "FFFFFF"},
    {"route_id": "R3", "agency_id": "MOCK", "route_short_name": "3",
     "route_long_name": "Route Three", "route_type": "3",
     "route_color": "009A44", "route_text_color": "FFFFFF"},
]

STOPS = [
    {"stop_id": "S_NORTH", "stop_name": "North Terminal", "stop_lat": "44.50", "stop_lon": "-73.25"},
    {"stop_id": "S_SOUTH", "stop_name": "South Terminal", "stop_lat": "44.45", "stop_lon": "-73.20"},
]

# ── trip / stop-time generators ───────────────────────────────────────────────

def make_calendar(start_date: str, end_date: str) -> list[dict]:
    return [
        {"service_id": "WD", "monday": "1", "tuesday": "1", "wednesday": "1",
         "thursday": "1", "friday": "1", "saturday": "0", "sunday": "0",
         "start_date": start_date, "end_date": end_date},
        {"service_id": "WE", "monday": "0", "tuesday": "0", "wednesday": "0",
         "thursday": "0", "friday": "0", "saturday": "1", "sunday": "1",
         "start_date": start_date, "end_date": end_date},
    ]


def make_feed_info(start_date: str, end_date: str, version: str) -> list[dict]:
    return [
        {"feed_publisher_name": "Test Transit Agency",
         "feed_publisher_url": "http://example.com",
         "feed_lang": "en",
         "feed_start_date": start_date,
         "feed_end_date": end_date,
         "feed_version": version}
    ]


def trips_and_stop_times(
    route_id: str,
    service_id: str,
    direction: int,
    hour_counts: dict[int, int],   # hour -> trips that depart in that hour
    start_stop: str,
    end_stop: str,
    trip_id_prefix: str,
) -> tuple[list[dict], list[dict]]:
    """Generate trip rows and stop_time rows for one route/service/direction."""
    trips_rows = []
    st_rows = []
    headsign = "South Terminal" if direction == 0 else "North Terminal"

    for hour, count in sorted(hour_counts.items()):
        spacing = 60 // max(count, 1)
        for i in range(count):
            tid = f"{trip_id_prefix}_{hour:02d}_{i:02d}"
            dep_min = i * spacing
            dep_time = f"{hour:02d}:{dep_min:02d}:00"
            arr_time_end = f"{hour:02d}:{dep_min + 20:02d}:00"

            trips_rows.append({
                "trip_id": tid,
                "route_id": route_id,
                "service_id": service_id,
                "trip_headsign": headsign,
                "direction_id": str(direction),
            })
            st_rows.extend([
                {"trip_id": tid, "arrival_time": dep_time,  "departure_time": dep_time,
                 "stop_id": start_stop, "stop_sequence": "1"},
                {"trip_id": tid, "arrival_time": arr_time_end, "departure_time": arr_time_end,
                 "stop_id": end_stop,   "stop_sequence": "2"},
            ])
    return trips_rows, st_rows


# ── schedule definitions ──────────────────────────────────────────────────────
#
# Format: {route_id: {direction: {hour: count, ...}}}
# We only define the interesting hours (non-zero).  Both directions get the
# same pattern for simplicity.

def r1_prev():
    return {
        0: {7: 4, 8: 4, 9: 4, 10: 2, 11: 2, 12: 2, 13: 2, 14: 2, 15: 2, 16: 2, 17: 4, 18: 4, 19: 4},
        1: {7: 4, 8: 4, 9: 4, 10: 2, 11: 2, 12: 2, 13: 2, 14: 2, 15: 2, 16: 2, 17: 4, 18: 4, 19: 4},
    }

def r1_curr():
    # AM peak: +2/hr (4→6).  PM shoulder: -1/hr (4→3)
    return {
        0: {7: 6, 8: 6, 9: 6, 10: 2, 11: 2, 12: 2, 13: 2, 14: 2, 15: 2, 16: 2, 17: 3, 18: 3, 19: 3},
        1: {7: 6, 8: 6, 9: 6, 10: 2, 11: 2, 12: 2, 13: 2, 14: 2, 15: 2, 16: 2, 17: 3, 18: 3, 19: 3},
    }

def r2_prev():
    return {
        0: {8: 2, 9: 2, 10: 2, 11: 2, 12: 2, 13: 2, 14: 2, 15: 2, 16: 2, 17: 2, 18: 2},
        1: {8: 2, 9: 2, 10: 2, 11: 2, 12: 2, 13: 2, 14: 2, 15: 2, 16: 2, 17: 2, 18: 2},
    }

def r2_curr():
    # +1 trip per hour all day
    return {
        0: {8: 3, 9: 3, 10: 3, 11: 3, 12: 3, 13: 3, 14: 3, 15: 3, 16: 3, 17: 3, 18: 3},
        1: {8: 3, 9: 3, 10: 3, 11: 3, 12: 3, 13: 3, 14: 3, 15: 3, 16: 3, 17: 3, 18: 3},
    }

def r3_prev():
    return {
        0: {9: 3, 10: 3, 11: 3, 12: 3, 13: 3, 14: 3, 15: 3, 16: 3, 17: 3},
        1: {9: 3, 10: 3, 11: 3, 12: 3, 13: 3, 14: 3, 15: 3, 16: 3, 17: 3},
    }

def r3_curr():
    # -1 trip per hour all day
    return {
        0: {9: 2, 10: 2, 11: 2, 12: 2, 13: 2, 14: 2, 15: 2, 16: 2, 17: 2},
        1: {9: 2, 10: 2, 11: 2, 12: 2, 13: 2, 14: 2, 15: 2, 16: 2, 17: 2},
    }


# ── build one GTFS dataset ────────────────────────────────────────────────────

def build_gtfs(label: str, start_date: str, end_date: str, version: str,
               schedules: dict) -> bytes:
    """
    schedules = {route_id: {direction: {hour: count}}}
    """
    route_stops = {
        "R1": ("S_NORTH", "S_SOUTH"),
        "R2": ("S_NORTH", "S_SOUTH"),
        "R3": ("S_NORTH", "S_SOUTH"),
    }
    all_trips = []
    all_st    = []

    for route_id, dirs in schedules.items():
        s, e = route_stops[route_id]
        for direction, hour_counts in dirs.items():
            prefix = f"{label}_{route_id}_D{direction}"
            t, st = trips_and_stop_times(
                route_id, "WD", direction, hour_counts, s, e, prefix
            )
            all_trips.extend(t)
            all_st.extend(st)

    files = {
        "agency.txt":    csv_bytes(AGENCY),
        "routes.txt":    csv_bytes(ROUTES),
        "stops.txt":     csv_bytes(STOPS),
        "calendar.txt":  csv_bytes(make_calendar(start_date, end_date)),
        "feed_info.txt": csv_bytes(make_feed_info(start_date, end_date, version)),
        "trips.txt":     csv_bytes(all_trips),
        "stop_times.txt":csv_bytes(all_st),
    }
    return make_zip(files)


# ── main ──────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    prev_zip = build_gtfs(
        label="prev",
        start_date="20260101", end_date="20260228", version="v1-prev",
        schedules={
            "R1": r1_prev(),
            "R2": r2_prev(),
            "R3": r3_prev(),
        },
    )
    curr_zip = build_gtfs(
        label="curr",
        start_date="20260301", end_date="20260430", version="v2-curr",
        schedules={
            "R1": r1_curr(),
            "R2": r2_curr(),
            "R3": r3_curr(),
        },
    )

    with open(os.path.join(OUT, "previous.zip"), "wb") as f:
        f.write(prev_zip)
    print("Wrote previous.zip")

    with open(os.path.join(OUT, "current.zip"), "wb") as f:
        f.write(curr_zip)
    print("Wrote current.zip")
