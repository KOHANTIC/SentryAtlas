import type { Event } from "./api.gen";
import { RANK } from "./features";
import { position } from "./filters";
import { distanceKm } from "./near";

export type SortBy = "time" | "severity" | "distance";

const rank = (e: Event) => (e.severity ? RANK[e.severity.level] : 0);
const updated = (e: Event) => Date.parse(e.time.updated);

/** Newest first, most severe first (newest first within a level), or nearest first to `from`. */
export function sortEvents(events: readonly Event[], by: SortBy, from?: [number, number]): Event[] {
  const out = [...events];
  if (by === "time") return out.sort((a, b) => updated(b) - updated(a));
  if (by === "distance") {
    if (!from) return out;
    const d = (e: Event) => {
      const p = position(e);
      return p ? distanceKm(from, p) : Infinity;
    };
    return out.sort((a, b) => d(a) - d(b));
  }
  return out.sort((a, b) => rank(b) - rank(a) || updated(b) - updated(a));
}
