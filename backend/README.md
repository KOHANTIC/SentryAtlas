# Disaster Watch — Backend

Go API server that aggregates natural disaster data from multiple public sources and exposes it through a single unified endpoint.

## Data Sources

| Source | Data | Upstream API |
|--------|------|-------------|
| USGS | Earthquakes | `earthquake.usgs.gov/fdsnws/event/1/query` |
| NASA EONET | Wildfires, volcanoes, storms, icebergs | `eonet.gsfc.nasa.gov/api/v3/events` |
| NOAA/NWS | Floods, storms, tornados, hurricanes, winter storms | `api.weather.gov/alerts/active` |
| GDACS | Earthquakes, cyclones, floods, volcanoes, droughts | `www.gdacs.org/gdacsapi/api/events/geteventlist/SEARCH` |

## Prerequisites

- Go 1.22+

## Getting Started

```bash
cd backend

# Install dependencies
go mod download

# Copy and optionally edit environment config
cp .env.example .env

# Run the server
go run ./cmd/server/
```

The server starts on `http://localhost:8080` by default.

## Configuration

All configuration is via environment variables. Every variable has a sensible default so none are required.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Port the HTTP server listens on |
| `CACHE_TTL_MINUTES` | `5` | How long upstream responses are cached in memory |
| `FETCH_TIMEOUT_SECONDS` | `30` | Max time to wait for upstream APIs to respond |

## API

### `GET /health`

Returns `{"status":"ok"}` — useful for uptime checks.

### `GET /api/v1/events`

Returns disaster events from all sources, merged and sorted by date (newest first).

#### Query Parameters

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `types` | string | *(all)* | Comma-separated event types to include (see list below) |
| `bbox` | string | *(none)* | Bounding box filter: `minLon,minLat,maxLon,maxLat` |
| `since` | string | *(none)* | Only events after this date — RFC 3339 (`2024-01-15T00:00:00Z`) or `YYYY-MM-DD` |
| `limit` | int | *(none)* | Max number of events to return (capped at 1000) |
| `format` | string | `geojson` | Response format: `geojson` or `json` |

#### Event Types

`earthquake`, `wildfire`, `volcano`, `storm`, `flood`, `cyclone`, `tornado`, `hurricane`, `winter_storm`, `tsunami`, `drought`, `iceberg`, `landslide`

#### Examples

```bash
# All events (GeoJSON)
curl "http://localhost:8080/api/v1/events"

# Earthquakes only, flat JSON format
curl "http://localhost:8080/api/v1/events?types=earthquake&format=json"

# Wildfires and floods since Jan 1 2025
curl "http://localhost:8080/api/v1/events?types=wildfire,flood&since=2025-01-01"

# Events in a bounding box around California, limit 50
curl "http://localhost:8080/api/v1/events?bbox=-124.48,32.53,-114.13,42.01&limit=50"
```

#### Response — GeoJSON (default)

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": { "type": "Point", "coordinates": [-117.5, 34.2] },
      "properties": {
        "id": "usgs-abc123",
        "title": "M 4.2 - 10km NW of ...",
        "event_type": "earthquake",
        "source": "usgs",
        "severity": "minor",
        "magnitude": 4.2,
        "started_at": "2024-01-15T08:30:00Z",
        "updated_at": "2024-01-15T09:00:00Z",
        "url": "https://earthquake.usgs.gov/..."
      }
    }
  ]
}
```

#### Response — Flat JSON (`?format=json`)

```json
{
  "events": [
    {
      "id": "usgs-abc123",
      "title": "M 4.2 - 10km NW of ...",
      "event_type": "earthquake",
      "source": "usgs",
      "coordinates": { "longitude": -117.5, "latitude": 34.2 },
      "magnitude": 4.2,
      "severity": "minor",
      "started_at": "2024-01-15T08:30:00Z",
      "updated_at": "2024-01-15T09:00:00Z",
      "url": "https://earthquake.usgs.gov/..."
    }
  ],
  "total": 1,
  "sources": ["usgs"]
}
```

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
│   │   └── gdacs.go                # GDACS
│   ├── cache/cache.go              # Generic in-memory TTL cache
│   ├── handler/events.go           # HTTP handler, query param parsing
│   ├── models/event.go             # Unified Event model, GeoJSON + flat JSON serialization
│   └── service/events.go           # Fan-out orchestration, merge, filter, caching
├── .env.example
├── go.mod
└── go.sum
```

## Architecture

All adapters are queried concurrently. If one upstream source fails, results from the others are still returned. Responses are cached in memory for the configured TTL to avoid hammering public APIs.

```
Client → chi Router → Events Handler → Events Service → In-Memory Cache
                                              ↓ (cache miss)
                                     ┌────────┼────────┐────────┐
                                   USGS    EONET     NOAA     GDACS
                                     └────────┼────────┘────────┘
                                          Merge + Filter + Sort
                                              ↓
                                    GeoJSON or JSON Response
```
