import tokens from "../../../brand/tokens.json";
import { PRESETS, type EventType, type Preset } from "./filters";

/** Everything a /map/ link carries. Absent fields mean the defaults. */
export type ViewState = {
  /** [longitude, latitude] of the map's center. */
  center?: [number, number];
  zoom?: number;
  bearing?: number;
  types?: EventType[];
  time: Preset;
  event?: string;
};

export const DEFAULT_TIME: Preset = "7d";
const TYPES = new Set(Object.keys(tokens.event));
const clamp = (x: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, x));
const round = (x: number, d: number) => Number(x.toFixed(d));
// Wraps a longitude into -180..180.
const wrap = (lon: number) => ((((lon + 180) % 360) + 360) % 360) - 180;

/**
 * Reads a query string such as `?at=20.1,10,2.3&types=flood,wildfire&time=30d&event=usgs-x`.
 * `at` is latitude, longitude, zoom and an optional bearing. Anything malformed is dropped,
 * never an error: a link should open the best view it can.
 */
export function parse(search: string): ViewState {
  const q = new URLSearchParams(search);
  const v: ViewState = { time: DEFAULT_TIME };
  const at = q.get("at")?.split(",").map(Number);
  if (at && at.length >= 3 && at.slice(0, 3).every(Number.isFinite)) {
    v.center = [wrap(at[1]), clamp(at[0], -85, 85)];
    v.zoom = clamp(at[2], 0, 22);
    if (at.length >= 4 && Number.isFinite(at[3]) && at[3] !== 0) v.bearing = ((at[3] % 360) + 360) % 360;
  }
  const types = q.get("types")?.split(",").filter((t) => TYPES.has(t)) as EventType[] | undefined;
  if (types?.length) v.types = [...new Set(types)];
  const time = q.get("time");
  if (time && time in PRESETS) v.time = time as Preset;
  const event = q.get("event");
  if (event && /^(usgs|eonet|noaa|gdacs)-.+/.test(event)) v.event = event;
  return v;
}

/** Writes a view as a query string, rounded so a link stays short, with defaults left out. */
export function serialize(v: ViewState): string {
  const q = new URLSearchParams();
  if (v.center && v.zoom !== undefined) {
    const parts = [round(v.center[1], 4), round(wrap(v.center[0]), 4), round(v.zoom, 2)];
    const bearing = v.bearing ? Math.round(v.bearing) % 360 : 0;
    q.set("at", (bearing ? [...parts, bearing] : parts).join(","));
  }
  if (v.types?.length) q.set("types", v.types.join(","));
  if (v.time !== DEFAULT_TIME) q.set("time", v.time);
  if (v.event) q.set("event", v.event);
  const s = q.toString().replaceAll("%2C", ",");
  return s ? `?${s}` : "";
}

// The URL is the state: components read it, and these two write it.
const CHANGE = "sentryatlas:url";

/** Replaces the current history entry: for moving the map, which would otherwise flood the back button. */
export function replaceView(v: ViewState) {
  const next = serialize(v);
  if (next === window.location.search) return;
  window.history.replaceState(window.history.state, "", window.location.pathname + next);
  window.dispatchEvent(new Event(CHANGE));
}

/** Adds a history entry: for selecting an event, so back closes it. */
export function pushView(v: ViewState) {
  window.history.pushState(window.history.state, "", window.location.pathname + serialize(v));
  window.dispatchEvent(new Event(CHANGE));
}

/** Subscribes to every change of the query string, ours and the browser's back and forward. */
export function subscribeURL(cb: () => void) {
  window.addEventListener(CHANGE, cb);
  window.addEventListener("popstate", cb);
  return () => {
    window.removeEventListener(CHANGE, cb);
    window.removeEventListener("popstate", cb);
  };
}
