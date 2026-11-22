# NWS zones

Most NWS alerts name forecast zones or counties and carry no shape. Two files place and draw them, built from NWS's own zone shapefiles by `backend/cmd/zones`.

| File | What it holds | Size |
|---|---|---|
| `backend/internal/zones/centers.json` | Each zone's center, `[lon, lat]` to 3 decimals, by zone id. Embedded in the server, which places an alert with no shape at the center of its zones. | 217 KB, 8,048 zones |
| `frontend/public/data/nws-zones.json` | Each zone's outline, a GeoJSON MultiPolygon with the zone id as the feature `id`, for drawing (#38) | 5.4 MB, 1.1 MB gzipped |

A test in `backend/internal/zones` holds the budget: under 300 KB and 6 MB.

## Source

The National Weather Service's zone shapefiles, at [weather.gov/gis](https://www.weather.gov/gis/), from 16 April 2026:

| File | Zones | Id |
|---|---|---|
| `z_16ap26` | Public forecast zones | state, `Z`, zone number: `ALZ019` |
| `mz16ap26` | Marine zones | its `ID`: `PZZ131` |
| `oz16ap26` | Offshore zones | its `ID`: `PZZ900` |
| `c_16ap26` | Counties | state, `C`, the county part of its FIPS code: `MEC029` |

These are the ids NWS alerts give in `affectedZones`. Every zone named by the 433 zone ids of the alerts recorded on 2026-11-22 is in the table.

NWS web information is in the public domain unless marked otherwise ([weather.gov/disclaimer](https://www.weather.gov/disclaimer)). Credit: National Weather Service.

## How the files are made

- **Centers:** each outer ring's area center, weighted by its area, across 180°.
  - The files' own `LON` and `LAT` are not used: for zones that cross 180° they are a naive average. Aleutians West comes out at -107.9°.
  - Everywhere else ours and NWS's agree to a median of 0.0004°.
- **Outlines:**
  - outer rings simplified to within 0.01° (about 1 km), rounded to 3 decimals;
  - holes are dropped;
  - a zone too small to survive simplification keeps its ring as drawn.

**Fire-weather zones** (`fz16ap26`) are left out. Their ids repeat public zone ids for other areas (`CAZ211` is both), so an alert's ids under `/zones/fire/` are listed but not used to place it.

## Refreshing

NWS publishes new dated files a few times a year.

1. Find the new names on the pages for public zones, marine zones, offshore zones and counties.
2. Put them in `defaultSources` in `backend/cmd/zones/main.go`.
3. From `backend/`, run:

   ```
   go run ./cmd/zones
   ```

   It downloads the four files and writes both outputs. It also takes zip paths or URLs as arguments, for files downloaded already.
4. Review the diff of `centers.json`, which has one zone a line, and run the tests.
