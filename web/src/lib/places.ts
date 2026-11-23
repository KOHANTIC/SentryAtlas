import { normalize, prefixOf } from "./placeKey.mjs";

/** A city from GeoNames: name, folded name, ISO country, longitude, latitude. */
export type Place = [string, string, string, number, number];

const files = new Map<string, Promise<Place[]>>();

/** The places whose name starts like the query, largest first. One small file per two letters, from this site. */
export async function searchPlaces(query: string, limit = 8, f: typeof fetch = fetch): Promise<Place[]> {
  const q = normalize(query);
  const prefix = prefixOf(q);
  if (!prefix) return [];
  let file = files.get(prefix);
  if (!file) {
    file = f(`/data/places/${prefix}.json`).then((r) => (r.ok ? (r.json() as Promise<Place[]>) : []), () => []);
    files.set(prefix, file);
  }
  return (await file).filter((p) => p[1].startsWith(q)).slice(0, limit);
}

let countries: Intl.DisplayNames | undefined;
/** A country's name in English, from the browser's own data. */
export function countryName(code: string): string {
  countries ??= new Intl.DisplayNames(["en"], { type: "region" });
  try {
    return countries.of(code) ?? code;
  } catch {
    return code;
  }
}
