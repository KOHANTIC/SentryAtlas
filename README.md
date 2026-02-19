# SentryAtlas

**Real-time disaster monitoring, open and free.**

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![Node](https://img.shields.io/badge/Node-22-339933?logo=node.js&logoColor=white)

SentryAtlas aggregates live disaster data from four trusted public sources and plots it on a single interactive map. Earthquakes, wildfires, floods, storms — see it all at a glance.

<!-- TODO: Add a screenshot of the map here -->

## Architecture

This is a monorepo with three components:

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────────────────────┐
│   Landing   │     │   Frontend   │────▶│            Backend              │
│  (static)   │     │  (Next.js)   │     │             (Go)               │
└─────────────┘     └──────────────┘     │                                │
                                         │  ┌─────┐ ┌─────┐ ┌────┐ ┌───┐ │
                                         │  │USGS │ │EONET│ │NOAA│ │GDA│ │
                                         │  │     │ │     │ │/NWS│ │CS │ │
                                         │  └─────┘ └─────┘ └────┘ └───┘ │
                                         └─────────────────────────────────┘
```

- **Backend** — Go API that fetches from 4 upstream sources concurrently, caches responses in memory, and serves a unified `/api/v1/events` endpoint in GeoJSON or flat JSON
- **Frontend** — Next.js app with MapLibre GL for interactive map rendering, filtering by event type and date range
- **Landing** — Static marketing site built with Next.js (`output: "export"`)

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 22+

### 1. Backend

```bash
cd backend
cp .env.example .env
go mod download
go run ./cmd/server/
# Server starts on http://localhost:8080
```

### 2. Frontend

```bash
cd frontend
cp .env.example .env.local
npm install
npm run dev
# App starts on http://localhost:3000
```

### 3. Landing

```bash
cd landing
npm install
npm run dev
# Landing page starts on http://localhost:3000
```

## Project Structure

```
sentryatlas/
├── backend/                 # Go API server
│   ├── cmd/server/          # Entry point
│   ├── internal/
│   │   ├── adapters/        # USGS, EONET, NOAA, GDACS integrations
│   │   ├── cache/           # Generic in-memory TTL cache
│   │   ├── handler/         # HTTP handler and query parsing
│   │   ├── models/          # Unified Event model
│   │   └── service/         # Fan-out orchestration
│   └── Dockerfile
├── frontend/                # Next.js map application
│   ├── src/
│   │   ├── app/             # App router pages
│   │   ├── components/      # MapView, FilterPanel, Legend, EventDetail
│   │   ├── hooks/           # Data fetching hooks
│   │   └── lib/             # API client, types, map styles
│   └── Dockerfile
├── landing/                 # Static landing page
│   └── src/app/             # Single-page marketing site
├── .do/app.yaml             # DigitalOcean App Platform spec
└── .github/                 # Issue & PR templates
```

## API

The backend exposes a single endpoint. See [`backend/README.md`](backend/README.md) for full documentation.

### `GET /api/v1/events`

Returns disaster events from all sources, merged and sorted by date (newest first).

| Param | Type | Description |
|-------|------|-------------|
| `types` | string | Comma-separated event types to include |
| `bbox` | string | Bounding box: `minLon,minLat,maxLon,maxLat` |
| `since` | string | Only events after this date (RFC 3339 or `YYYY-MM-DD`) |
| `limit` | int | Max events to return (capped at 1000) |
| `format` | string | `geojson` (default) or `json` |

**Event types:** `earthquake`, `wildfire`, `volcano`, `storm`, `flood`, `cyclone`, `tornado`, `hurricane`, `winter_storm`, `tsunami`, `drought`, `iceberg`, `landslide`

## Data Sources

| Source | Data | URL |
|--------|------|-----|
| USGS | Earthquakes | [earthquake.usgs.gov](https://earthquake.usgs.gov) |
| NASA EONET | Wildfires, volcanoes, storms, icebergs | [eonet.gsfc.nasa.gov](https://eonet.gsfc.nasa.gov) |
| NOAA / NWS | Floods, tornadoes, hurricanes, winter storms | [weather.gov](https://www.weather.gov) |
| GDACS | Cyclones, droughts, floods, volcanoes | [gdacs.org](https://www.gdacs.org) |

## Deployment

The project includes Docker configuration and a DigitalOcean App Platform spec.

### Docker

```bash
# Backend
docker build -t sentryatlas-backend ./backend
docker run -p 8080:8080 sentryatlas-backend

# Frontend
docker build --build-arg NEXT_PUBLIC_API_URL=http://localhost:8080 -t sentryatlas-frontend ./frontend
docker run -p 3000:3000 sentryatlas-frontend
```

### DigitalOcean App Platform

The `.do/app.yaml` defines all three components. Deploy by connecting the repo in the DigitalOcean dashboard or via the CLI:

```bash
doctl apps create --spec .do/app.yaml
```

## Contributing

Contributions are welcome! See [`CONTRIBUTING.md`](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE).

---

Made by the [KOHANTIC](https://kohantic.com) team.
