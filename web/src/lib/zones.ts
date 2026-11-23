// NWS zone outlines (docs/data/nws-zones.md): 5.4 MB, so fetched once, the first
// time the map has a zone-only alert to draw.

type Polygons = number[][][][];
export type ZoneShapes = Map<string, Polygons>;

let shapes: ZoneShapes | undefined;
let loading: Promise<void> | undefined;
const listeners = new Set<() => void>();

/** Starts the one fetch of the outlines; listeners hear when they arrive. */
export function loadZones(url = "/data/nws-zones.json"): Promise<void> {
  loading ??= fetch(url)
    .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`zones: ${r.status}`))))
    .then((fc: { features: { id: string; geometry: { type: string; coordinates: Polygons | number[][][] } }[] }) => {
      shapes = new Map(fc.features.map((f) => [f.id, f.geometry.type === "Polygon" ? [f.geometry.coordinates as number[][][]] : (f.geometry.coordinates as Polygons)]));
      listeners.forEach((l) => l());
    })
    .catch(() => {
      loading = undefined; // a later need tries again
    });
  return loading;
}

export const getZones = () => shapes;

export function subscribeZones(cb: () => void) {
  listeners.add(cb);
  return () => listeners.delete(cb);
}
