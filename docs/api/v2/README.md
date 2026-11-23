# SentryAtlas API v2

v2 is served at `/api/v2/snapshot` and `/api/v2/events/{id}`, beside v1, which stays (see `backend/README.md`, Compatibility).

| File | What it is |
|---|---|
| `schema.json` | JSON Schema (2020-12) of a snapshot and its events |
| `snapshot.example.json` | A small snapshot built from real records of 2026-11-22, one or two per source |
| `backend/internal/event/` | The same model in Go. Its tests hold the schema's fields and the example to it |

## Endpoints

Both are read from copies the server polls on its own, never from a source while you wait. They allow any origin (with `ETag` readable by a page's scripts), share v1's rate limit, and answer `503` with `Cache-Control: no-store` until every source has been polled once after a start.

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

### The waiting list

A waiting list for location alerts. These two routes are not part of the read API above: they accept requests only from the project's own pages (`WAITLIST_ORIGINS`), have their own limits, and answer `503` while the list is not configured. Every answer carries `Cache-Control: no-store`.

**`POST /api/v2/waitlist`** with

```json
{ "email": "person@example.org", "area": "Sacramento, CA", "hazards": ["flood", "wildfire"], "news": false }
```

- **Fields:** `email` is required. `area` is free text up to 100 characters. `hazards` lists event types (below). `news` is a separate consent to product news, never set unless the person ticks it.
- **Checks:** the address must parse, carry no display name, and belong to a domain that receives mail. An unknown field is refused.
- **What happens:** an email goes out with a link to `https://sentryatlas.com/waitlist/confirm/#<token>`. The token is signed, expires after 48 hours, and carries the whole request, so the server keeps nothing between the two steps. It sits after `#`, which browsers never send to a server.
- **Answers:**
  - `202 {"status":"sent"}`, the same whether or not the address was seen before, including when that address has had its three links for the day;
  - `400` with an `error`;
  - `429` past five signups an hour from one client;
  - `503` once the day's 90 sends are used;
  - `503` with an `error` when the email could not be sent.

**`POST /api/v2/waitlist/confirm`** with `{ "token": "<token>" }`, sent by the confirmation page when the person presses its button, so a mail scanner opening the link confirms nothing.

- **What happens:** the contact is saved at the mail provider with each consent as its own topic: the waiting list always, product news only if it was ticked.
- **Answers:**
  - `200 {"status":"confirmed"}`, again on a second confirmation;
  - `400 {"status":"invalid"}`;
  - `410 {"status":"expired"}`;
  - `503 {"status":"failed"}` when the provider could not save it;
  - `429` past 20 an hour from one client.

## Events

- **`id`** is `<source>-<upstream id>` and stays the same when the source updates the event:
  - USGS: its event id.
  - EONET: its event id.
  - GDACS: `<type>-<event id>`.
  - NWS: the alert's first message, so an update keeps the id. The messages of one alert (its updates, and parallel messages for different zones, found by their shared VTEC event) are one event.
- **`group_id`** is shared by reports of one real event from different sources, and **`primary`** marks the one to draw. Both are in the snapshot only, worked out again on every build: never store a group id as a key. An event with no twin has neither.
  - **Quakes:** GDACS repeats USGS's. A GDACS quake within 30 s, 50 km and 0.5 magnitude of a USGS quake is that quake, each matched once, closest first. On 2026-11-22 the pairs were a median 0.5 s, 0.8 km and 0.0 magnitude apart, and these limits paired 516 of 519.
  - **EONET:** an event is grouped with the GDACS report it lists among its sources.
  - **Primary:** the member from the source first in USGS, GDACS, NWS, EONET.
  - Storms, fires and floods from different sources are not matched by place.
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
  - An NWS alert issued for named zones, with no shape of its own, is centered on its zones: the mean of their centers, from NWS's zone shapefiles ([`docs/data/nws-zones.md`](../../data/nws-zones.md)). Fire-weather zones are not placed.
- **`geometry`** is the full GeoJSON shape when the source gives one: a Point, an EONET track as a LineString, or an NWS alert's Polygon or MultiPolygon.
- **`zones`** are NWS zone ids, such as `LMZ844`.
- **Precision:** coordinates keep 5 decimals, about a meter.
- **Size:** a geometry has at most 1,000 points. A larger shape is simplified, or left out with the center kept.
