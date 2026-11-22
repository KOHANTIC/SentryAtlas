# SentryAtlas API v2: the event model

v2 is not served yet. `/api/v2/snapshot` and `/api/v2/events/{id}` arrive with #23, and v1 stays (see `backend/README.md`, Compatibility).

| File | What it is |
|---|---|
| `schema.json` | JSON Schema (2020-12) of a snapshot and its events |
| `snapshot.example.json` | A small snapshot built from real records of 2026-11-22, one or two per source |
| `backend/internal/event/` | The same model in Go. Its tests hold the schema's fields and the example to it |

## Events

- **`id`** is `<source>-<upstream id>` and stays the same when the source updates the event:
  - USGS: its event id.
  - EONET: its event id.
  - GDACS: `<type>-<event id>`.
  - NWS: the alert's first message, so an update keeps the id.
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
| GDACS | its severity unit (`km/h`, `km2`, ...) |

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
