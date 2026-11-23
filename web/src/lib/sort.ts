import type { Event } from "./api.gen";
import { RANK } from "./features";

export type SortBy = "time" | "severity";

const rank = (e: Event) => (e.severity ? RANK[e.severity.level] : 0);
const updated = (e: Event) => Date.parse(e.time.updated);

/** Newest first, or most severe first with the newest first within a level. */
export function sortEvents(events: readonly Event[], by: SortBy): Event[] {
  const out = [...events];
  if (by === "time") return out.sort((a, b) => updated(b) - updated(a));
  return out.sort((a, b) => rank(b) - rank(a) || updated(b) - updated(a));
}
