# SentryAtlas API v2

v2 is served at `/api/v2/snapshot` and `/api/v2/events/{id}`, beside v1, which stays (see `backend/README.md`, Compatibility).

| File | What it is |
|---|---|
| `schema.json` | JSON Schema (2020-12) of a snapshot and its events |
| `snapshot.example.json` | A small snapshot built from real records of 2026-11-22, one or two per source |
| `backend/internal/event/` | The same model in Go. Its tests hold the schema's fields and the example to it |

## Endpoints

Both are read from copies the server polls on its own, never from a source while you wait. They allow any origin, share v1's rate limit, and answer `503` with `Cache-Control: no-store` until every source has been polled once after a start.

### `GET /api/v2/snapshot`

The whole world in one response that every visitor shares, for a client to filter itself.

- **`window`:** `7d` (the default) or `30d`, the events whose `time.updated` falls in that many days. Any other parameter or value is a `400`, so the responses stay few enough for a cache to keep.
- **Contents:** a snapshot as in `schema.json`. Events leave out `description` and `details.noaa.instruction`, which only the detail endpoint carries. `sources[].count` counts this snapshot's events.
- **Built once per poll:** the server builds and gzips it when a poll is recorded, or after a minute so expired events leave, not per request.
- **Caching:**
  - `Cache-Control: public, max-age=30` and `Vary: Accept-Encoding`;
  - gzip when the client accepts it;
  - a weak `ETag`, and `If-None-Match` with it answers `304`. The tag leaves out `generated`, so it changes only when the data does.

### `GET /api/v2/events/{id}`

One event in full, with its long text and full geometry, and its source's status:

```json
{ "event": { "id": "usgs-ak0261d6m9t2", "...": "..." }, "source": { "source": "usgs", "ok": true, "fetched_at": "2026-11-22T21:54:57Z", "stale": false, "count": 9443 } }
```

`404` when no event with that id is being served. `Cache-Control: public, max-age=30`.

## Events

- **`id`** is `<source>-<upstream id>` and stays the same when the source updates the event:
  - USGS: its event id.
  - EONET: its event id.
  - GDACS: `<type>-<event id>`.
  - NWS: the alert's first message, so an update keeps the id. The messages of one alert (its updates, and parallel messages for different zones, found by their shared VTEC event) are one event.
- **`group_id`** is shared by reports of one real event from different sources. It is worked out again on every fetch, so never store it as a key.
- **`type`** is one of the 15 v1 types.
- **`description`**, and an NWS alert's **`details.noaa.instruction`**, are long. Only the event-detail endpoint carries them.
- **`details`** holds exactly the part named by `source`: `usgs`, `eonet`, `noaa` or `gdacs`.

## Severity

One scale for every source, with the source's own word in `source_level`:

| `level` | USGS PAGER | NWS | GDACS | EONET |
|---|---|---|---|---|
| `extreme` | red | Extreme | Red | none |
| `severe` | orange | Severe | Orange | none |
| `moderate` | yellow | Moderate | none | none |
| `minor` | green | Minor | Green | none |

An event with no level from its source has no `severity`.

## Magnitude

A `value` and the `unit` the source gives:

| Source | Unit |
|---|---|
| USGS | the magnitude type (`mww`, `ml`, `md`, ...) |
| EONET | `kts`, `acres` or `NM^2` |
| GDACS | its severity unit: `M` (earthquakes), `ha` (wildfires), `km/h` (cyclones), `km2` (droughts); none for floods and volcanoes |

## Time

All in UTC, each with one meaning:

| Field | Meaning | USGS | EONET | NWS | GDACS |
|---|---|---|---|---|---|
| `started` | when the event began | origin time | first observation | onset (or effective, or sent) | `fromdate` |
| `updated` | when the source last changed its record | `updated` | latest observation | latest message's `sent` | `datemodified` |
| `ends` | when the source expects it over | none | none | `ends` | `todate` |
| `expires` | when the record stops being valid | none | none | `expires` | none |

## Location

- **`center`** is `[longitude, latitude]`. It is set for every event that can be placed.
  - An NWS alert issued for named zones gets its center from the zones' shapes (#25). Until then it carries only `zones`.
- **`geometry`** is the full GeoJSON shape when the source gives one: a Point, an EONET track as a LineString, or an NWS alert's Polygon or MultiPolygon.
- **`zones`** are NWS zone ids, such as `LMZ844`.
- **Precision:** coordinates keep 5 decimals, about a meter.
- **Size:** a geometry has at most 1,000 points. A larger shape is simplified, or left out with the center kept.
