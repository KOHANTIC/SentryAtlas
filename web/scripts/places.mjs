// Writes public/data/places/<prefix>.json from GeoNames' cities15000.txt
// (docs/data/places.md). Run from web/: node scripts/places.mjs <cities15000.txt>
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { normalize, prefixOf } from "../src/lib/placeKey.mjs";

const src = process.argv[2];
if (!src) {
  console.error("usage: node scripts/places.mjs <path to cities15000.txt>");
  process.exit(2);
}

/** One place: name, ISO country code, longitude, latitude, population. */
export function parseLine(line) {
  const f = line.split("\t");
  if (f.length < 15) return undefined;
  const [, name, ascii, , lat, lon, , , country, , , , , , population] = f;
  const la = Number(lat), lo = Number(lon), pop = Number(population);
  if (!name || !Number.isFinite(la) || !Number.isFinite(lo)) return undefined;
  return { name, key: normalize(ascii || name), country, lon: Math.round(lo * 1000) / 1000, lat: Math.round(la * 1000) / 1000, pop: Number.isFinite(pop) ? pop : 0 };
}

const byPrefix = new Map();
for (const line of readFileSync(src, "utf8").split("\n")) {
  const p = line && parseLine(line);
  if (!p || p.key.length < 2) continue;
  const prefix = prefixOf(p.key);
  if (!byPrefix.has(prefix)) byPrefix.set(prefix, []);
  byPrefix.get(prefix).push(p);
}

const out = new URL("../public/data/places/", import.meta.url);
rmSync(out, { recursive: true, force: true });
mkdirSync(out, { recursive: true });
let n = 0;
for (const [prefix, list] of byPrefix) {
  list.sort((a, b) => b.pop - a.pop || a.key.localeCompare(b.key));
  // [name, folded name, country, lon, lat], largest first.
  writeFileSync(new URL(`${prefix}.json`, out), JSON.stringify(list.map((p) => [p.name, p.key, p.country, p.lon, p.lat])));
  n += list.length;
}
console.log(`${n} places in ${byPrefix.size} files`);
