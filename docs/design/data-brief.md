# Data brief: what each source can and cannot show

What the data supports, for the design rounds (#29, #30). Read live from `/api/v2/snapshot?window=30d` and `/api/v1/events?format=sse` on 2026-11-22 at 22:55 UTC, after M2. The model is in [`docs/api/v2/README.md`](../api/v2/README.md).

## At world zoom

Events updated within each window, and the markers left once grouped duplicates are drawn once (#26):

| Window | Events | Markers | USGS | EONET | NWS | GDACS |
|---|---|---|---|---|---|---|
| 24 hours | 846 | 830 | 343 | 1 | 178 | 324 |
| 7 days | 3,708 | 3,607 | 2,907 | 16 | 181 | 604 |
| 30 days | 11,330 | 10,817 | 9,451 | 43 | 181 | 1,655 |

USGS quakes are three quarters or more of every view, most of them small (`ml` and `md` magnitudes). A world view has to cope with thousands of points; clustering or a magnitude floor is not optional.

## Per source

| | USGS | EONET | NWS | GDACS |
|---|---|---|---|---|
| **What** | Earthquakes, plus quarry blasts, explosions and the like (`other`, 320) | Wildfires, icebergs, storms | US weather alerts | Quakes, wildfires, floods, droughts, cyclones, volcanoes, worldwide |
| **Shape** | Point | Point (31), track as a line (12) | Polygon (16, 9%); the rest name zones only | Point |
| **No location** | 0% | 0% | 0%, but 91% are placed at their zones' center (#25) | 0% |
| **Severity** | PAGER level on 83 of 9,451 (1%), only for significant quakes | None | Extreme, Severe, Moderate, Minor: 24, 47, 107 and 3 with none | Green on 1,648 (99.6%), Orange 4, Red 3 |
| **Magnitude** | Always, in its own type (`ml`, `md`, `mb`, `mww`...) | Acres, knots or NM² | None | `M`, `ha`, `km/h`, `km2`; none for floods and volcanoes |
| **Times** | Started, updated; no end | First and latest observation; no end | Started, updated, ends (90%), expires (100%) | Started, updated, ends |
| **Text** | None beyond the title ("M 4.5 - 10 km SW of ...") | Description on 21% | Description and instructions, always, long | One short line, always |
| **Polled every** | 5 minutes | 10 minutes | 2 minutes | 10 minutes |
| **Window** | The past 30 days | Open events updated in 30 days (7,257 are open; most have no observation in a month) | Alerts in force now | The past 30 days |

## What follows for the design

- **Severity cannot carry the map alone.** Only NWS grades most of its events. USGS grades 1% of quakes, EONET none, and GDACS calls nearly everything Green. A legend that colors by severity shows almost one color. Type, plus magnitude for quakes, does more work.
- **Warning areas are zones, not drawn shapes.** Nine in ten NWS alerts name forecast zones or counties. The map can draw those zones from `nws-zones.json` (#38), but the outline is the zones', not a forecaster's polygon.
- **Detail is uneven.** An NWS alert has paragraphs of text and instructions. A USGS quake has a title and numbers. A GDACS event has one line. The detail panel must look complete with little text.
- **Tracks are rare.** 12 EONET storms and icebergs have a path; everything else is a point.
- **Duplicates are grouped, not removed.** 514 GDACS quakes are USGS quakes. The snapshot marks one primary per group; the other report can be shown as "also reported by".

## Hard limits

- **Only NWS alerts in force are known.** Past warnings cannot be replayed, and an alert is gone the moment NWS cancels or expires it.
- **An event is gone once it leaves its feed:**
  - USGS and GDACS after 30 days;
  - EONET when it closes;
  - nothing is kept across a deploy.
- **A link to an event that has gone needs an "ended" state.** The event-detail endpoint answers 404 for it.
- **Fire-weather zones are not placed.** Their ids repeat public zone ids for other areas, so such an alert keeps its zones but has no center (#25).
- **Freshness is per source.** Each source has its own `fetched_at`, and one can be stale while the others are current; `/health/stale` alarms past 30 minutes to 2 hours. A partial-data state is a normal state, not an error.
