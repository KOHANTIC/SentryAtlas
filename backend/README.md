# SentryAtlas — Backend

Go API server that aggregates natural disaster data from multiple public sources and exposes it through a single unified endpoint.

## Data Sources

| Source | Data | Upstream API |
|--------|------|-------------|
| USGS | Earthquakes | `earthquake.usgs.gov/fdsnws/event/1/query` |
| NASA EONET | Wildfires, volcanoes, storms, icebergs | `eonet.gsfc.nasa.gov/api/v3/events` |
| NOAA/NWS | Floods, storms, tornados, hurricanes, winter storms | `api.weather.gov/alerts/active` |
| GDACS | Earthquakes, cyclones, floods, volcanoes, droughts | `www.gdacs.org/gdacsapi/api/events/geteventlist/SEARCH` |

## Prerequisites

- Go 1.27+

## Getting Started

```bash
cd backend
go run ./cmd/server/
```

The server starts on `http://localhost:8080`. It reads its settings from the environment, not from a `.env` file; `.env.example` lists them.

## Configuration

Every variable has a default, so none is required. A number that is set but not a positive integer stops the server at startup.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Port the HTTP server listens on |
| `CACHE_TTL_MINUTES` | `5` | How long each source's response is kept in memory |
| `FETCH_TIMEOUT_SECONDS` | `30` | How long to wait for a source to answer |
| `RATE_LIMIT_PER_MINUTE` | `60` | Requests per minute to `/api/v1` from one client address (IPv6 grouped by /64) |
| `ALLOWED_ORIGINS` | `*` | Comma-separated CORS origins |
| `NWS_USER_AGENT` | `SentryAtlas/1.0 (github.com/KOHANTIC/SentryAtlas)` | User-Agent sent to api.weather.gov, which asks for one that identifies the caller. Forks should set their own. |
| `CLIENT_IP_HEADER` | empty | Header the proxy in front of the API writes the client address into, overwriting any value a client sends. The rate limit keys on it. Empty: the TCP peer, for running with no proxy. |

## API

### `GET /health`

Returns `{"status":"ok"}`. It is not rate-limited.

### `GET /api/v1/events`

Events from all sources, merged and sorted newest first by `started_at`. Events that start at the same moment come in no particular order.

#### Query parameters

| Param | Default | Description |
|-------|---------|-------------|
| `types` | all | Comma-separated event types (list below). An unknown type is a 400. |
| `bbox` | none | `minLon,minLat,maxLon,maxLat`. Events without coordinates are left out. A box over the date line works written either way: unwrapped as a map sends it (`170,-30,190,0`) or with the west edge east of the east edge (`170,-30,-170,0`). 360° wide or more is the whole world. NaN, infinite values or a latitude outside -90..90 is a 400. |
| `since` | none | Only events starting at or after this time: RFC 3339 (`2026-08-09T00:00:00Z`) or `YYYY-MM-DD`, read as UTC. It narrows each source's window (below) and cannot reach past it. |
| `limit` | 500 for `geojson` and `json`, none for `sse` | Most events to return. Above 1000 is treated as 1000. |
| `format` | `geojson` | `geojson`, `json` or `sse`. Anything else is a 400. |

#### Windows

Each source is fetched for the same window whatever the request, and every request is cut from that copy:

| Source | Window |
|--------|--------|
| USGS | The past month of earthquakes of every magnitude (USGS's `all_month` summary feed) |
| NASA EONET | Every event EONET lists as open, in every category |
| NOAA/NWS | Every active alert |
| GDACS | Every event type GDACS covers, from 30 days ago to today |

#### Event types

`earthquake`, `wildfire`, `volcano`, `storm`, `flood`, `cyclone`, `tornado`, `hurricane`, `winter_storm`, `tsunami`, `drought`, `iceberg`, `landslide`, `weather`, `other`

`weather` is the NWS fallback for an alert with no more specific class; `other` covers upstream categories no adapter maps yet.

#### Examples

```bash
# All events (GeoJSON)
curl "http://localhost:8080/api/v1/events"

# Earthquakes only, flat JSON
curl "http://localhost:8080/api/v1/events?types=earthquake&format=json"

# Wildfires and floods since 9 August 2026
curl "http://localhost:8080/api/v1/events?types=wildfire,flood&since=2026-08-09"

# Events in a box around California, at most 50
curl "http://localhost:8080/api/v1/events?bbox=-124.48,32.53,-114.13,42.01&limit=50"
```

#### Response: GeoJSON (default)

`Content-Type: application/geo+json`. An event without coordinates, common for NWS alerts that name a region, has `"geometry": null`.

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": { "type": "Point", "coordinates": [-151.9234, 59.7568] },
      "properties": {
        "id": "usgs-us7000red1",
        "title": "M 6.2 - 35 km W of Anchor Point, Alaska",
        "event_type": "earthquake",
        "source": "usgs",
        "severity": "extreme",
        "magnitude": 6.2,
        "started_at": "2026-08-10T00:00:00Z",
        "updated_at": "2026-08-10T01:00:00Z",
        "url": "https://earthquake.usgs.gov/earthquakes/eventpage/us7000red1",
        "metadata": { "depth": 42.5, "place": "35 km W of Anchor Point, Alaska" }
      }
    }
  ],
  "sources": [
    { "source": "eonet", "ok": true },
    { "source": "gdacs", "ok": true },
    { "source": "noaa", "ok": false, "error": "..." },
    { "source": "usgs", "ok": true }
  ]
}
```

`sources` is a GeoJSON foreign member; GeoJSON readers ignore it.

#### Response: flat JSON (`format=json`)

`Content-Type: application/json`. `coordinates` is `null` for an event without them.

```json
{
  "events": [
    {
      "id": "usgs-us7000red1",
      "title": "M 6.2 - 35 km W of Anchor Point, Alaska",
      "event_type": "earthquake",
      "source": "usgs",
      "coordinates": { "longitude": -151.9234, "latitude": 59.7568 },
      "magnitude": 6.2,
      "severity": "extreme",
      "started_at": "2026-08-10T00:00:00Z",
      "updated_at": "2026-08-10T01:00:00Z",
      "url": "https://earthquake.usgs.gov/earthquakes/eventpage/us7000red1",
      "metadata": { "depth": 42.5, "place": "35 km W of Anchor Point, Alaska" }
    }
  ],
  "total": 13,
  "sources": [
    { "source": "usgs", "ok": true }
  ]
}
```

#### Response: Server-Sent Events (`format=sse`)

`Content-Type: text/event-stream`. Each source's events arrive in their own frame as soon as that source answers, so the first events show without waiting for the slowest source:

```
event: features
data: {"type":"FeatureCollection","features":[...]}

: keep-alive

event: done
data: {"total":13,"sources":[{"source":"usgs","ok":true}, ...]}
```

A `features` frame is a FeatureCollection for one source, without `sources`. A source that fails or has no matching events sends no `features` frame. A `: keep-alive` comment comes every 15 seconds while sources are still answering. The `done` frame comes last, with the total and every source's status, and then the stream closes. `limit` applies across the whole stream, to whichever sources answer first.

#### Fields

| Field | Always present | Notes |
|-------|----------------|-------|
| `id` | yes | `<source>-<upstream id>` |
| `title`, `event_type`, `source` | yes | |
| `started_at`, `updated_at` | yes | RFC 3339 with the offset the source gave: NWS alerts carry their area's offset, such as `-05:00`; the other sources are in UTC. GeoJSON gives whole seconds; flat JSON adds fractional seconds when there are any. An event whose source gives no start time has `0001-01-01T00:00:00Z`. |
| `description` | no | |
| `magnitude` | no | A number with no unit. |
| `severity` | no | `extreme`, `severe`, `moderate` or `minor` |
| `url` | no | The source's page for the event |
| `metadata` | no | Extra fields that differ by source, such as `depth` and `place` (USGS), `area_desc` and `urgency` (NWS), or `alert_level` and `country` (GDACS) |

`sources` lists every source asked, each as `{"source", "ok"}`, with `"error"` only when `ok` is `false`. A partial result can be told apart from a complete one by it. `error` is a short reason for people, such as `timed out`, `answered HTTP 503`, `sent a response SentryAtlas could not read` or `could not be reached`.

#### Status codes

| Status | When | Body |
|--------|------|------|
| 200 | Any source answered, or `format=sse` | The response above |
| 400 | A parameter is invalid | `{"error": "<what is wrong>"}` |
| 429 | Over the rate limit; `Retry-After` and `X-RateLimit-*` headers say when to retry | `{"error": "rate limit exceeded"}` |
| 502 | `geojson` or `json`, and every source asked failed | `{"error": "all upstream sources failed"}` |

A 200 in `geojson` or `json` carries `Cache-Control: public, max-age=60`; the stream carries `no-cache`.

## Compatibility

`/api/v1` is public, and the people using it may never read this repository. What it keeps:

- **Kept:** routes, parameters, status codes, field names, JSON types, which fields can be `null`, and id formats.
- **May change:** new fields may appear, and values may become more correct. Error message text is for people, not for parsing.
- **Recorded:** `internal/handler/testdata/v1/shape.txt` lists every field v1 returns and its JSON types; the golden files beside it hold whole responses. A pull request that changes `shape.txt` changes the API and says why.

Made in #4, each within the rules above:

- A `since` earlier than a source's window returns only what is inside the window.
- A request without `since` gets each source's whole window, so USGS now gives 30 days where it gave 7. With the default `limit` of 500 the newest 500 are the same, since a week holds far more quakes than that.
- EONET events in categories SentryAtlas does not map yet (`other`) are fetched for every request, and are returned when `other` is asked for.

Made in #5: `sources[].error` is a short reason, without upstream addresses or Go error text.

Made in #6: a box over the date line returns the events on both sides of it. A box with NaN, an infinite value or a latitude outside -90..90 answers 400, where it answered 200 with nothing.

Announced changes:

- **#19:** EONET's `started_at` becomes the event's first observation, not its latest.
- **#22:** a 502 only happens before the first fetch after a start; after that, a failing source is served from its last good copy and reported in `sources`.

## Tests

```bash
go test -race ./...
```

`internal/handler/golden_test.go` runs the whole chain (adapters, cache, service and handler) against the recorded upstream responses in `internal/adapters/testdata/` and compares each answer with `internal/handler/testdata/v1/`. After a deliberate change of values, rewrite the recorded answers and review the diff:

```bash
go test ./internal/handler -run Golden -update
```

`-update-shape` rewrites `shape.txt`, which is a change to the API (see Compatibility).

## Project Structure

```
backend/
├── cmd/server/main.go              # Entry point, wiring, graceful shutdown
├── internal/
│   ├── adapters/
│   │   ├── adapter.go              # Adapter interface, FetchParams, BBox
│   │   ├── usgs.go                 # USGS Earthquake Hazards
│   │   ├── eonet.go                # NASA EONET v3
│   │   ├── noaa.go                 # NOAA/NWS Alerts
│   │   ├── gdacs.go                # GDACS
│   │   └── testdata/               # Recorded upstream responses
│   ├── cache/cache.go              # Generic in-memory TTL cache
│   ├── handler/events.go           # HTTP handler, query param parsing
│   ├── handler/testdata/v1/        # The v1 contract: golden responses and shape.txt
│   ├── httpx/httpx.go              # Request log, rate-limit key, 429 answer
│   ├── models/event.go             # Unified Event model, GeoJSON + flat JSON serialization
│   └── service/events.go           # Fan-out orchestration, merge, filter, caching
├── .env.example
├── go.mod
└── go.sum
```

## Architecture

Each source is fetched for its fixed window and kept in memory for `CACHE_TTL_MINUTES`, under the source's name alone, so every request in that time is answered from the same copy and the sources hear from the API once per TTL, not once per visitor. Concurrent requests share one fetch. The sources are fetched concurrently; if one fails, results from the others are still returned.

A source's records are read one at a time: a record that cannot be read is skipped and logged, and the rest of that source is kept. A source counts as failed when it answers anything but 200, sends more than 32 MB, sends nothing readable, or when reading it crashes, which is recovered and logged with its stack rather than ending the process.

```
Client → chi Router → Events Handler → Events Service → In-Memory Cache (one entry per source)
                                              ↓ (expired)
                                     ┌────────┼────────┐────────┐
                                   USGS    EONET     NOAA     GDACS
                                     └────────┼────────┘────────┘
                                          Merge + Filter + Sort
                                              ↓
                                    GeoJSON or JSON Response
```
