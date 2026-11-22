# SentryAtlas — Backend

Go API server that aggregates natural disaster data from multiple public sources and exposes it through a single unified endpoint.

## Data Sources

| Source | Data | Upstream API |
|--------|------|-------------|
| USGS | Earthquakes | `earthquake.usgs.gov/fdsnws/event/1/query` |
| NASA EONET | Wildfires, volcanoes, storms, icebergs | `eonet.gsfc.nasa.gov/api/v3/events` |
| NOAA/NWS | Floods, storms, tornados, hurricanes, winter storms | `api.weather.gov/alerts/active` |
| GDACS | Earthquakes, cyclones, floods, volcanoes, droughts, wildfires | `www.gdacs.org/gdacsapi/api/events/geteventlist/SEARCH` |

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
| `FETCH_TIMEOUT_SECONDS` | `30` | How long one poll waits for its source to answer |
| `RATE_LIMIT_PER_MINUTE` | `60` | Requests per minute to `/api/v1` from one client address (IPv6 grouped by /64) |
| `ALLOWED_ORIGINS` | `*` | Comma-separated CORS origins |
| `NWS_USER_AGENT` | `SentryAtlas/1.0 (github.com/KOHANTIC/SentryAtlas)` | User-Agent sent to api.weather.gov, which asks for one that identifies the caller. Forks should set their own. |
| `CLIENT_IP_HEADER` | empty | Header the proxy in front of the API writes the client address into, overwriting any value a client sends. The rate limit keys on it. Empty: the TCP peer, for running with no proxy. |

## API

### `GET /health`

Each source is polled on its own schedule (NWS every 2 minutes, USGS every 5, EONET and GDACS every 10), and `/health` reports where that stands. It answers `503` with `"status":"starting"` until every source has been polled once, then `200` with `"status":"ok"`, so a new deploy takes traffic only once it has data. A source that fails is reported, not a reason for `503`. It is not rate-limited or cached.

```json
{
  "status": "ok",
  "sources": [
    { "source": "usgs", "ok": true, "stale": false, "age_seconds": 74, "count": 9444 },
    { "source": "noaa", "ok": false, "stale": true, "age_seconds": 305, "count": 261, "error": "timed out" }
  ]
}
```

- **`age_seconds`:** how old the copy in use is; absent if the source has never answered.
- **`stale`:** the last poll failed, or sent an implausible copy, and an older copy is in use.
- **NWS limit:** an NWS copy over an hour old is withdrawn.

### `GET /health/stale`

For an outside monitor. Answers `503` with `"status":"stale"` while any source's copy is older than its limit, or it has none yet, and `200` with `"status":"ok"` otherwise. `stale` names the sources past their limit. Not rate-limited or cached.

```json
{ "status": "stale", "stale": ["noaa"] }
```

| Source | Limit | Polled every |
|---|---|---|
| NWS | 30 minutes | 2 minutes |
| USGS | 1 hour | 5 minutes |
| EONET, GDACS | 2 hours | 10 minutes |

### `GET /api/v2/snapshot` and `GET /api/v2/events/{id}`

v2: one shared, cacheable snapshot of every source (`window=7d` or `30d`), and each event in full. [`docs/api/v2/README.md`](../docs/api/v2/README.md) documents both, with the event model and its schema.

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
| GDACS | Every event type GDACS covers, at every alert level, from 30 days ago to today |

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

A `features` frame is a FeatureCollection for one source, without `sources`. A source with no copy to serve, or no matching events, sends no `features` frame. A `: keep-alive` comment comes every 15 seconds while the stream is open. The stream stops when the client leaves. The `done` frame comes last, with the total and every source's status, and then the stream closes. `limit` applies across the whole stream, to the sources in the order they are sent.

#### Fields

| Field | Always present | Notes |
|-------|----------------|-------|
| `id` | yes | `<source>-<upstream id>` |
| `title`, `event_type`, `source` | yes | |
| `started_at`, `updated_at` | yes | RFC 3339 in UTC. GeoJSON gives whole seconds; flat JSON adds fractional seconds when there are any. An event whose source gives no start time has `0001-01-01T00:00:00Z`. |
| `description` | no | |
| `magnitude` | no | An earthquake's magnitude, with no unit; USGS events only. |
| `severity` | no | `extreme`, `severe`, `moderate` or `minor` |
| `url` | no | The source's page for the event |
| `metadata` | no | Extra fields that differ by source, such as `depth` and `place` (USGS), `area_desc` and `urgency` (NWS), or `alert_level` and `country` (GDACS) |

`sources` lists every source asked, each as `{"source", "ok"}`. A partial result can be told apart from a complete one by it.

- **`error`:** only when `ok` is `false`.
- **`stale`:** `true` when the last poll failed and an older copy is served.
- **`fetched_at`:** when the copy served was fetched; absent if the source has not answered since the server started. `error` is a short reason for people, such as `timed out`, `answered HTTP 503`, `sent a response SentryAtlas could not read` or `could not be reached`.

#### Status codes

| Status | When | Body |
|--------|------|------|
| 200 | Any source answered, or `format=sse` | The response above |
| 400 | A parameter is invalid | `{"error": "<what is wrong>"}` |
| 429 | Over the rate limit; `Retry-After` and `X-RateLimit-*` headers say when to retry | `{"error": "rate limit exceeded"}` |
| 502 | `geojson` or `json`, and no source asked has answered since the server started | `{"error": "all upstream sources failed"}` |

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

Made in #17:
- USGS events that are not earthquakes stop being labelled `earthquake`. Quarry blasts, explosions and ice quakes are `other`; landslides are `landslide`. A request for `other` or `landslide` now includes USGS.
- A USGS event without a depth has no `metadata.depth`, where it had `0`.

Made in #18:
- NWS alerts that have been updated are included. An update replaces the message it updates, so asking for alerts alone missed them: about a third of active alerts.
- One event per alert. The messages that carry one alert (its updates, and parallel messages for different zones) are merged, so their zones, times and severity come together. Its `id` is the alert's first message, and it no longer changes when the alert is updated.
- NWS times are in UTC, where they kept the alert area's offset.
- An alert's point is the area center of its polygon. The old average counted the closing point twice and leaned toward it.
- Fire-weather products (Red Flag Warning, Fire Weather Watch) are `weather`, not `wildfire`. A Hurricane Force Wind Warning, a marine product, is `weather`, not `hurricane`.

Made in #19:
- EONET's `started_at` is the event's first observation, and `updated_at` its latest, where both were the latest.
- `magnitude` is only set for USGS earthquakes. It has no unit, and EONET's values are acres, knots or square nautical miles, which would read as a quake's size. v2 carries them with their units. EONET's v1 magnitude was empty in practice, since EONET puts it where the old code did not look.

Made in #20:
- GDACS's Green alerts are included, and every page of results is read. GDACS returns only Orange and Red alerts unless asked, so the window held 7 events where it holds about 1,700, most of them Green wildfires and earthquakes.
- GDACS's `updated_at` is when GDACS last modified the event, where it was the event's end date.
- `metadata.severity_value` and `metadata.severity_unit` carry GDACS's severity (`M`, `ha`, `km/h`, `km2`), which was always empty: GDACS puts it where the old code did not look. Floods and volcanoes have none.
- `metadata.country` lists the affected countries separated by `, `, without the trailing `, ` GDACS adds.
- A GDACS event given as a polygon is placed at its area center, where it was dropped.

Made in #22:
- A 502 only happens before any source asked has answered since the server started. After that, a failing source is served from its last good copy, with `ok: false`, `stale: true` and the reason.
- `sources[]` adds `stale` and `fetched_at`.
- Requests are answered from copies the server polls on its own (see Architecture), so none waits on a source.

Made in #25:
- An NWS alert that names zones but carries no shape now has `coordinates`: the mean of its zones' centers ([`docs/data/nws-zones.md`](../docs/data/nws-zones.md)). About nine in ten active alerts are like this, so most NWS events gain a point and now match a `bbox`.

## Tests

```bash
go test -race ./...
```

`internal/adapters/testdata/live/` holds real responses from the four sources, trimmed to one record of each kind they sent (event class, geometry, status, unit, alert level), and a test requires every record to decode. To record them again from the live sources:

```bash
RECORD_LIVE=1 go test ./internal/adapters -run RecordLive -count=1
```

The other files in `internal/adapters/testdata/` are written by hand, for edge cases the real data may not show on a given day.

`internal/handler/golden_test.go` runs the whole chain (adapters, store, service and handler) against the hand-written responses in `internal/adapters/testdata/` and compares each answer with `internal/handler/testdata/v1/`. After a deliberate change of values, rewrite the recorded answers and review the diff:

```bash
go test ./internal/handler -run Golden -update
```

`-update-shape` rewrites `shape.txt`, which is a change to the API (see Compatibility).

## Monitoring

The fixtures only prove the adapters against the day they were recorded. Two checks watch the real thing.

- **Weekly live source check** (`.github/workflows/sources.yml`, Mondays and by hand):
  - runs the adapters against the live sources;
  - fails when a source does not answer, a record cannot be read, an event breaks the model, or a field every recorded record has is gone from every live one;
  - new fields are listed in the run summary, not failed;
  - a failure opens an issue labelled `source check`, or comments on the open one;
  - it is not part of the `gate` check. To run it locally:

  ```bash
  LIVE_CONTRACT=1 go test ./internal/adapters -run 'TestLiveContract$' -count=1 -v
  ```
- **Outside monitor on `/health/stale`:** set up by the maintainer outside this repo. It alarms on any answer but `200`, checked every few minutes with a retry or two, since a new deploy answers `503` for the seconds before its first polls.

## Project Structure

```
backend/
├── cmd/server/main.go              # Entry point, wiring, graceful shutdown
├── cmd/zones/                      # Builds the NWS zone centers and outlines from NWS's shapefiles
├── internal/
│   ├── adapters/
│   │   ├── adapter.go              # Adapter interface, FetchParams, BBox
│   │   ├── usgs.go                 # USGS Earthquake Hazards
│   │   ├── eonet.go                # NASA EONET v3
│   │   ├── noaa.go                 # NOAA/NWS Alerts
│   │   ├── gdacs.go                # GDACS
│   │   └── testdata/               # Recorded upstream responses
│   ├── handler/events.go           # HTTP handler, query param parsing
│   ├── handler/health.go           # /health: readiness and each source's copy
│   ├── handler/v2.go               # v2: the snapshot, built once per poll, and event detail
│   ├── handler/testdata/v1/        # The v1 contract: golden responses and shape.txt
│   ├── httpx/httpx.go              # Request log, rate-limit key, 429 answer
│   ├── models/event.go             # Unified Event model, GeoJSON + flat JSON serialization
│   ├── service/events.go           # v1 answers from the store: merge, filter, sort, limit
│   ├── store/                      # Each source's last good copy, and its poller
│   └── zones/                      # NWS zone centers, embedded, for alerts with no shape
├── .env.example
├── go.mod
└── go.sum
```

## Architecture

One poller per source fetches its fixed window on its own schedule (NWS every 2 minutes, USGS every 5, EONET and GDACS every 10, each with some jitter) and hands the copy to the store. Requests read the store and never call a source, so the sources hear from the API on that schedule whoever is visiting. Fetches take turns, so one response is decoded at a time.

The store replaces a copy only with a plausible one: not empty after a full one, and not under a tenth of a copy of 100 or more, unless it comes three polls in a row. A failed poll keeps the last copy, served as stale, and retries after 30 seconds, doubling up to the source's interval. Events past their expiry are not served, and an NWS copy over an hour old is withdrawn.

A source's records are read one at a time: a record that cannot be read is skipped and logged, and the rest of that source is kept. A source counts as failed when it answers anything but 200, sends more than 32 MB, sends nothing readable, or when reading it crashes, which is recovered and logged with its stack rather than ending the process.

```
Pollers (one per source, taking turns)          Client → chi Router → Events Handler → Events Service
  USGS · EONET · NOAA · GDACS                                                             ↓
            ↓ plausible copies                                                   Store (last good copy each)
          Store ──────────────────────────────────────────────────────────────→ Merge + Filter + Sort
                                                                                         ↓
                                                                              GeoJSON, JSON or SSE response
```
