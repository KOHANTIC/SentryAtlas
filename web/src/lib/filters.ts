import type { Event, Snapshot, Source } from "./api.gen";

export type EventType = Event["type"];
/** West, south, east, north in degrees. West may exceed east, or the box run past ±180, across the date line. */
export type BBox = [number, number, number, number];

export type Filters = {
  /** The types to keep; absent keeps every type. */
  types?: ReadonlySet<EventType>;
  /** Keep events that began at or after this time (ms since the epoch). */
  since?: number;
  /** Keep events placed inside this box; events with no place are left out. */
  bbox?: BBox;
};

const HOUR = 3_600_000;

/** The time presets, and the snapshot window each needs. */
export const PRESETS = {
  "24h": { window: "7d", ms: 24 * HOUR },
  "7d": { window: "7d", ms: 7 * 24 * HOUR },
  "30d": { window: "30d", ms: 30 * 24 * HOUR },
} as const satisfies Record<string, { window: Snapshot["window"]; ms: number }>;
export type Preset = keyof typeof PRESETS;

/** Where an event is drawn: its center, or its geometry when that is a point. */
export function position(e: Event): [number, number] | undefined {
  const l = e.location;
  if (l?.center) return l.center;
  if (l?.geometry?.type === "Point") return l.geometry.coordinates;
  return undefined;
}

/** Whether a point lies in a box, which may cross the date line. */
export function inBBox([lon, lat]: [number, number], [west, south, east, north]: BBox): boolean {
  if (lat < south || lat > north) return false;
  const width = east - west;
  if (width >= 360) return true;
  // How far east of the west edge the point is, wrapped to 0..360.
  const offset = (((lon - west) % 360) + 360) % 360;
  return width >= 0 ? offset <= width : offset <= width + 360;
}

/** The events that pass every filter, one per group: the member the API marks primary. */
export function filterEvents(events: readonly Event[], f: Filters): Event[] {
  return events.filter((e) => {
    if (e.group_id && !e.primary) return false;
    if (f.types && !f.types.has(e.type)) return false;
    if (f.since !== undefined && Date.parse(e.time.started) < f.since) return false;
    if (f.bbox) {
      const p = position(e);
      if (!p || !inBBox(p, f.bbox)) return false;
    }
    return true;
  });
}

export type SourceFreshness = {
  source: Source;
  ok: boolean;
  /** The last fetch failed and an older copy is shown. */
  stale: boolean;
  /** Whole minutes since the source last answered; absent if it never has. */
  silentMinutes?: number;
  count: number;
};

/** How fresh each source's part of a snapshot is, and whether any part is missing or old. */
export function freshness(s: Snapshot, now: number): { partial: boolean; sources: SourceFreshness[] } {
  const sources = s.sources.map((x) => ({
    source: x.source,
    ok: x.ok,
    stale: x.stale,
    silentMinutes: x.fetched_at ? Math.max(0, Math.floor((now - Date.parse(x.fetched_at)) / 60_000)) : undefined,
    count: x.count,
  }));
  return { partial: sources.some((x) => !x.ok || x.stale), sources };
}
