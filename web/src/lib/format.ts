import tokens from "../../../brand/tokens.json";
import type { Event, Source } from "./api.gen";

export const SOURCE_NAMES: Record<Source, string> = { usgs: "USGS", eonet: "NASA EONET", noaa: "NWS", gdacs: "GDACS" };

/** A time in the viewer's zone, with UTC beside it unless the viewer is in UTC. */
export function formatTime(iso: string, timeZone?: string): string {
  const d = new Date(iso);
  const when = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit", timeZone }).format(d);
  // US English names zones the way people read them (PST, not GMT-8).
  const zoneName = new Intl.DateTimeFormat("en-US", { timeZoneName: "short", timeZone }).formatToParts(d).find((p) => p.type === "timeZoneName")?.value;
  const local = zoneName ? `${when} ${zoneName}` : when;
  const zone = timeZone ?? Intl.DateTimeFormat().resolvedOptions().timeZone;
  if (zone === "UTC" || zone === "Etc/UTC") return local;
  const utc = new Intl.DateTimeFormat("en-GB", { hour: "2-digit", minute: "2-digit", timeZone: "UTC" }).format(d);
  return `${local} (${utc} UTC)`;
}

/** A magnitude in its own unit: "M 5.2 (mww)" for a quake, "120 kts" for wind. */
export function formatMagnitude(e: Pick<Event, "type" | "magnitude">): string | undefined {
  const m = e.magnitude;
  if (!m) return undefined;
  if (e.type === "earthquake") return `M ${m.value.toFixed(1)} (${m.unit})`;
  return `${m.value.toLocaleString("en")} ${m.unit}`;
}

/** The severity in words, with the source's own word beside it. */
export function formatSeverity(e: Pick<Event, "severity" | "source">): string {
  if (!e.severity) return tokens.severity.ungraded.word;
  const word = tokens.severity[e.severity.level].word;
  return `${word} (${SOURCE_NAMES[e.source]}: ${e.severity.source_level})`;
}

/** The type's name from the tokens. */
export const typeLabel = (type: Event["type"]) => tokens.event[type]?.label ?? type;
