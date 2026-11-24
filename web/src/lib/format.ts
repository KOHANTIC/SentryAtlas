import tokens from "../../../brand/tokens.json";
import type { Event, Source } from "./api.gen";
import type { Preset, SourceFreshness } from "./filters";

export const SOURCE_NAMES: Record<Source, string> = { usgs: "USGS", eonet: "NASA EONET", noaa: "NWS", gdacs: "GDACS", tsunami: "NWS tsunami" };

export const PRESET_LABELS: Record<Preset, string> = { "24h": "24 hours", "7d": "7 days", "30d": "30 days" };

/** "36 hours", "1 hour", "2.5 days": how far back the window reaches. */
export function spanLabel(hours: number): string {
  if (hours < 48) return hours === 1 ? "1 hour" : `${hours} hours`;
  const days = Math.round((hours / 24) * 10) / 10;
  return `${days} days`;
}

/** The time window a link shows, as words: its narrowed span, or else its preset. */
export const windowLabel = (time: Preset, hours?: number) => (hours ? spanLabel(hours) : PRESET_LABELS[time]);

/** What to say about each source that is missing or behind: its name, then what is wrong. */
export function sourceNotes(sources: readonly SourceFreshness[]): { name: string; note: string }[] {
  return sources
    .filter((s) => !s.ok || s.stale)
    .map((s) => ({
      name: SOURCE_NAMES[s.source],
      note: s.stale && s.silentMinutes !== undefined ? `has not answered for ${s.silentMinutes} minutes; showing its last update.` : "is unavailable.",
    }));
}

/** A day and time without the year, for a row: "22 Nov, 14:27". */
export const formatShort = (iso: string) =>
  new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" }).format(new Date(iso));

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

/**
 * A source's text as paragraphs, every word kept: blank lines part paragraphs,
 * and the single line breaks NWS wraps its text with become spaces.
 */
export function paragraphs(text: string): string[] {
  return text
    .split(/\n\s*\n/)
    .map((p) => p.split("\n").map((l) => l.trim()).filter(Boolean).join(" "))
    .filter(Boolean);
}
