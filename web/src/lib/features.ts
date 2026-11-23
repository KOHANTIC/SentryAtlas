import tokens from "../../../brand/tokens.json";
import type { Event } from "./api.gen";
import { position } from "./filters";
import type { Fill } from "./marks";

type Level = NonNullable<Event["severity"]>["level"];

/** Severity as a number, so a cluster can keep the worst inside it: 0 is not graded. */
export const RANK: Record<Level, number> = { minor: 1, moderate: 2, severe: 3, extreme: 4 };

export type MarkProps = {
  id: string;
  type: Event["type"];
  fill: Fill;
  rank: number;
  /** An earthquake's magnitude, which sizes its ring. */
  mag?: number;
};

type Feature = { type: "Feature"; geometry: { type: "Point"; coordinates: [number, number] }; properties: MarkProps };
export type MarkCollection = { type: "FeatureCollection"; features: Feature[] };

/**
 * The map's two sources: ungraded earthquakes, drawn as rings and clustered at
 * low zoom because they are most of the map, and every other placed event,
 * drawn as its type's mark with its severity's fill.
 */
export function toFeatures(events: readonly Event[]): { quakes: MarkCollection; marks: MarkCollection } {
  const quakes: Feature[] = [];
  const marks: Feature[] = [];
  for (const e of events) {
    const at = position(e);
    if (!at) continue;
    const level = e.severity?.level;
    const props: MarkProps = {
      id: e.id,
      type: e.type,
      fill: (level ? tokens.severity[level].fill : tokens.severity.ungraded.fill) as Fill,
      rank: level ? RANK[level] : 0,
    };
    if (e.type === "earthquake" && e.magnitude) props.mag = e.magnitude.value;
    const f: Feature = { type: "Feature", geometry: { type: "Point", coordinates: at }, properties: props };
    (e.type === "earthquake" && !level ? quakes : marks).push(f);
  }
  return { quakes: { type: "FeatureCollection", features: quakes }, marks: { type: "FeatureCollection", features: marks } };
}
