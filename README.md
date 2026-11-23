# SentryAtlas

**Real-time disaster monitoring, open and free.**

[![CI](https://github.com/KOHANTIC/SentryAtlas/actions/workflows/ci.yml/badge.svg)](https://github.com/KOHANTIC/SentryAtlas/actions/workflows/ci.yml)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Node](https://img.shields.io/badge/Node-22-339933?logo=node.js&logoColor=white)

SentryAtlas aggregates live disaster data from four trusted public sources and plots it on a single interactive map. Earthquakes, wildfires, floods, storms — see it all at a glance.

**[sentryatlas.com](https://sentryatlas.com)** · **[Open the map](https://map.sentryatlas.com)**

![The SentryAtlas map, showing seven days of global events](docs/screenshot.png)

## Architecture

Three components, deployed as two DigitalOcean apps:

```
   sentryatlas.com                     map.sentryatlas.com
        │                                      │
   ┌────┴─────┐                    ┌───────────┴───────────┐
   │ Landing  │                    │ Frontend  │  /api ──▶ │ Backend
   │ (static) │                    │ (Next.js) │           │  (Go)
   └──────────┘                    └───────────┴───────────┘
                                                     │
                        ┌───────────┬───────────┬────┴──────┐
                        │   USGS    │   EONET   │   NOAA    │  GDACS
                        │           │           │   /NWS    │
                        └───────────┴───────────┴───────────┘
```

- **Backend** — Go API that polls 4 upstream sources on its own schedule, keeps each one's last good copy in memory, and serves a unified `/api/v1/events` endpoint as GeoJSON, flat JSON, or a Server-Sent Events stream
- **Frontend** — Next.js app with MapLibre GL for interactive map rendering, filtering by event type, time range, and viewport
- **Landing** — Static marketing site built with Next.js (`output: "export"`)

## Quick Start

### Prerequisites

- Go 1.27+
- Node.js 22+

### 1. Backend

```bash
cd backend
go run ./cmd/server/
# Server starts on http://localhost:8080
# Settings come from the environment; backend/.env.example lists them
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
npm run dev -- --port 3002
# Landing page starts on http://localhost:3002
# (both Next.js apps default to port 3000 — give one of them another port)
```

## Project Structure

```
sentryatlas/
├── backend/                 # Go API server
│   ├── cmd/server/          # Entry point
│   ├── internal/
│   │   ├── adapters/        # USGS, EONET, NOAA, GDACS integrations
│   │   ├── event/           # The v2 event model and its v1 projection
│   │   ├── handler/         # HTTP handlers and query parsing
│   │   ├── models/          # The v1 event model, event-type registry
│   │   ├── service/         # v1 answers from the store
│   │   └── store/           # Each source's last good copy, and its poller
│   └── Dockerfile
├── frontend/                # Next.js map application
│   ├── src/
│   │   ├── app/             # App router pages
│   │   ├── components/      # MapView, FilterPanel, Legend
│   │   ├── hooks/           # Data fetching hooks
│   │   └── lib/             # API client, types, map styles
│   └── Dockerfile
├── landing/                 # Static landing page
│   └── src/app/             # Single-page marketing site
├── .do/                     # DigitalOcean App Platform specs
│   ├── app.yaml             # Backend + frontend (map.sentryatlas.com)
│   └── landing.yaml         # Landing site (sentryatlas.com)
└── .github/                 # CI workflow, issue & PR templates
```

## API

One public endpoint, `GET /api/v1/events`, returns the events from all four sources merged, newest first, as GeoJSON, flat JSON, or a Server-Sent Events stream that sends each source's events as soon as it answers. Every response says which sources answered, so a partial result can be told apart from a complete one. No key is needed.

```bash
curl "https://map.sentryatlas.com/api/v1/events?types=earthquake&format=json&limit=10"
```

[`backend/README.md`](backend/README.md) is the reference: parameters, response shapes, status codes, the rate limit, and what v1 promises to keep.

## Data Sources

| Source | Data | URL |
|--------|------|-----|
| USGS | Earthquakes | [earthquake.usgs.gov](https://earthquake.usgs.gov) |
| NASA EONET | Wildfires, volcanoes, storms, icebergs | [eonet.gsfc.nasa.gov](https://eonet.gsfc.nasa.gov) |
| NOAA / NWS | Floods, tornadoes, hurricanes, winter storms | [weather.gov](https://www.weather.gov) |
| GDACS | Cyclones, droughts, floods, volcanoes, earthquakes | [gdacs.org](https://www.gdacs.org) |

All four are public and keyless. The one key SentryAtlas holds is its email provider's, for the waiting list, and it stores no personal data itself: the list is kept by the provider ([privacy](https://sentryatlas.com/privacy/)).

## Deployment

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

Two apps, one spec each:

```bash
doctl apps create --spec .do/app.yaml       # backend + frontend
doctl apps create --spec .do/landing.yaml   # landing site
```

Both specs set `deploy_on_push` on `main`, so a merge deploys the code. A push does not apply the specs themselves: a change to a spec, such as a new environment variable, is applied with `doctl apps update <app-id> --spec ...` or in the DigitalOcean dashboard, before the change that needs it is merged.

### Cloudflare in front

Both hosts are proxied by Cloudflare. App Platform does not cache responses from the API, and Cloudflare does not cache JSON on its own, so one Cache Rule makes it cache the v2 snapshot (`/api/v2/snapshot`), which says how long it may be kept:

- **Match:** `(http.request.uri.path eq "/api/v2/snapshot")`. The default cache key keeps the query string, so `window=7d` and `window=30d` are cached apart.
- **Cache eligibility:** Eligible for cache.
- **Edge TTL:** use the `Cache-Control` header if present, bypass the cache if not. The snapshot sends `public, max-age=30` with an ETag, so the edge keeps it 30 seconds, then revalidates with the API.
- **Browser TTL:** respect the origin's TTL. Otherwise the zone's default (4 hours) replaces `max-age=30` and browsers keep an old map.

Before its first polls the API answers the snapshot with `503` and `Cache-Control: no-store`, which this rule never caches. The SSE stream on `/api/v1/events` is outside the rule and passes through uncached.

To check it (the first request after 30 seconds reads `MISS` or `REVALIDATED`, the next `HIT`; a repeat with `If-None-Match` set to the ETag gets `304` from the edge):

```bash
curl -s -o /dev/null -D - "https://map.sentryatlas.com/api/v2/snapshot?window=7d" | grep -i -E "cf-cache-status|cache-control|age|etag"
```

To undo it, turn the rule off in the zone's Cache Rules; every request then reaches the API again (`cf-cache-status: DYNAMIC`).

## Contributing

Contributions are welcome! See [`CONTRIBUTING.md`](CONTRIBUTING.md) for guidelines, and [`SECURITY.md`](SECURITY.md) for reporting vulnerabilities.

## License

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE).

---

Made by the [KOHANTIC](https://kohantic.com) team.
