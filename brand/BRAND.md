# SentryAtlas brand

The look is the Survey Sheet, chosen in round 1 (#29) and drawn through every state in round 2 (#30): a printed survey map, light and plain, with a quiet dark theme. Every design value is in [`tokens.json`](tokens.json); this file says what they mean. The new site (#32) reads its CSS variables and its map style from that file and from nowhere else.

## Name

**SentryAtlas**: one word, capital S and A. Labels set it in capitals through the type (`SENTRYATLAS`); the text itself is never typed in capitals. Not "Sentry Atlas", not "Sentryatlas".

## Logo

The sheet mark: a framed map with a coastline and a point on it. Frame and point in `ink`, coastline in `action`.

| File | Use |
|---|---|
| [`logo/mark.svg`](logo/mark.svg) | The mark alone, on a clear ground. Follows the device's light or dark setting, so it serves as the favicon. |
| [`logo/icon.svg`](logo/icon.svg) | The mark on a light `sheet` tile, for app icons and share images. |

- **With the name:** the mark at 1.2 times the cap height, then the name in the label face at weight 800, with 0.08em tracking (`font.label.wordmark-tracking`).
- **Clear space:** a quarter of the mark's width on every side.
- **Smallest size:** 16 px. Below that, use the name alone.
- **Do not** recolor it outside the tokens, add effects, or set it on a map without a `sheet` ground behind it.

It replaces the contour mark (rings cut by a gutter), which Kyro retired for this one on 2026-11-22 (#30). The two current apps keep the contour mark until they are removed (#40).

## Color

Two themes with the same names. Light is the default; dark follows the device setting, and a page may set `data-theme` to override it. `node brand/css.mjs` prints them as CSS variables.

| Token | Role | Light | Dark |
|---|---|---|---|
| `paper` | Page ground | `#f3f6f2` | `#11181c` |
| `sheet` | Panels, cards, the sheet on the map | `#fbfcfa` | `#172026` |
| `ink` | Text, frames, the selected mark's leader | `#17242c` | `#e3eae6` |
| `muted` | Secondary text | `#4b5e68` | `#9db0b5` |
| `rule` | Hairlines between rows (decorative) | `#c7d2cb` | `#2c3b41` |
| `action` | Links, buttons, the focus ring, the logo's coast | `#0b5a76` | `#7cc3dc` |
| `on-action` | Text on a button | `#ffffff` | `#0c1a20` |
| `ocean`, `land` | Map grounds | `#d9e7ea`, `#edf1e6` | `#13232b`, `#1d2b2b` |
| `coast`, `grid` | Coastline and graticule (decorative) | `#93a99b`, `#c4d6da` | `#3f5752`, `#1c303a` |
| `warn`, `warn-bg` | The notice that a source is stale or down: its rule and ground | `#8a5a00`, `#fbf1dc` | `#f0c36a`, `#2a2416` |

Text reaches 4.5:1 on its ground and marks and functional rules 3:1, in both themes; `tokens.test.mjs` computes every pair. The dark theme is a deep blue-green rather than black, with the event colors lightened to keep their contrast.

## Type

| Face | Use | Licence |
|---|---|---|
| **Public Sans** (400 to 700) | All running text, 16 px at 1.55 line height | SIL Open Font License 1.1, from the US Web Design System |
| **Sofia Sans Extra Condensed** (600 to 800) | Labels, the wordmark, headings set in capitals, figures | SIL Open Font License 1.1, by Lettersoup |

Labels are set in capitals with 0.12em tracking (`font.label.tracking`); never type a label in capitals. Figures use tabular numerals. The site serves both faces itself; no page loads them from a font host.

## Events

Each of the 15 types the API sends has a color and a shape. Color says what kind of event it is; shape keeps apart the types whose colors some reader cannot tell apart. The rule, which the test holds in both themes: two types that come closer than 8 in OKLab (x100), under normal sight or a simulated protanopia, deuteranopia or tritanopia, never share a shape. Every mark reaches 3:1 on the sheet, the land and the ocean.

| Type | Shape | Light | Dark |
|---|---|---|---|
| Earthquake | circle, its ring sized by magnitude | `#6f3a96` | `#b98fdc` |
| Volcano | triangle | `#9b1c1c` | `#f06d6d` |
| Tsunami | triangle | `#2c5071` | `#7191b0` |
| Iceberg | triangle | `#22808c` | `#64c4ce` |
| Flood | triangle pointing down, the surveyor's water-level sign | `#1b689c` | `#6db3e4` |
| Tornado | triangle pointing down | `#3d396e` | `#8385ad` |
| Landslide | triangle pointing down | `#a86e31` | `#fbc593` |
| Storm | hexagon | `#4a43a0` | `#9d97ea` |
| Cyclone | hexagon | `#9c2e66` | `#e27caa` |
| Winter storm | hexagon | `#307fb4` | `#7bcaeb` |
| Wildfire | diamond | `#b8420e` | `#f08a52` |
| Other | diamond | `#5f6b74` | `#a5b0b8` |
| Drought | square | `#8a6712` | `#d4ae57` |
| Hurricane | square | `#6a3a3e` | `#b97a82` |
| Weather | square | `#4f5f80` | `#96a6c6` |

- **The circle is the earthquake's alone.** Quakes are most of the map, drawn as thin rings at 70% opacity with a radius of `(magnitude − 1.5) × 1.1` px at world zoom, never under 1.2 px. Any other circle would be lost among them.
- **Marks are 10 px** at world zoom, with a 1.4 px stroke.
- The type's name is printed wherever the mark appears in text: the list, the key, the open event.

## Severity

How full the mark is, and the level's word printed beside it. Never color alone.

| Level | Mark | Word |
|---|---|---|
| Extreme | filled, with a halo ring 3 px outside | EXTREME |
| Severe | filled | SEVERE |
| Moderate | half filled, the left half | MODERATE |
| Minor | hollow: a `sheet` fill inside the stroke | MINOR |
| Not graded | a smaller dotted outline, so it never reads as Minor | NOT GRADED |

Most events are not graded: only NWS grades most of its own, and EONET grades none ([data brief](../docs/design/data-brief.md)). A graded quake draws its ring with its level's fill.

## Voice

SentryAtlas reads like the legend of a good map: plain, exact and calm. It says what a source reported, where and when, in the source's own words and units, and names the source every time. It adds no alarm and no comfort of its own: no "breaking", no "devastating", no exclamation marks. An agency's warning is shown word for word, and SentryAtlas never sounds like the agency: it is not an official warning service, and it links to the official warning. When something is missing or old, it says so and says how old. Short sentences, active voice, numbers with their units, times with their zone.

## Changing a value

Edit `tokens.json`, then run the checks:

```
node --test brand/tokens.test.mjs
```

CI runs the same checks in the `brand` job, part of `gate`. A new event type in the API's schema fails them until it has a color and a shape here.
