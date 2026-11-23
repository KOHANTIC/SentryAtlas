# Places

The map's place search reads `web/public/data/places/<prefix>.json`, built from GeoNames, so nothing typed leaves the browser.

| | |
|---|---|
| Source | GeoNames [`cities15000.zip`](https://download.geonames.org/export/dump/): every city of 15,000 people or more, 34,248 on 2026-11-23 |
| Licence | [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Credited in the map's credit line and, with the licence, on `/data/` |
| Files | One per first two letters of a name's folded form (lowercase, no accents, letters and digits only): 403 files, the largest 82 KB |
| Each place | `[name, folded name, ISO country code, longitude, latitude]` to 3 decimals, largest population first |

Country names come from the browser (`Intl.DisplayNames`), not from a file.

## Refreshing

1. Download `cities15000.zip` from GeoNames and unzip it.
2. From `web/`:

   ```
   node scripts/places.mjs <path to cities15000.txt>
   ```

   It rewrites `public/data/places/`. Review the count it prints and the diff.
