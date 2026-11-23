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

type AreaFeature = { type: "Feature"; properties: { id: string; type: Event["type"] }; geometry: { type: "MultiPolygon"; coordinates: number[][][][] } };
type TrackFeature = { type: "Feature"; properties: { id: string; type: Event["type"] }; geometry: { type: "LineString"; coordinates: number[][] } };

/** Whether an alert can only be drawn from its zones: it names some and has no shape of its own. */
export const zoneOnly = (e: Event) => !!e.location?.zones?.length && !["Polygon", "MultiPolygon"].includes(e.location.geometry?.type ?? "");

/**
 * The events' areas: their own polygons, and for zone-only alerts the outlines
 * of the zones they name once those have loaded. Zones it does not know are skipped.
 */
export function toAreas(events: readonly Event[], zones?: Map<string, number[][][][]>) {
  const features: AreaFeature[] = [];
  for (const e of events) {
    const g = e.location?.geometry;
    const props = { id: e.id, type: e.type };
    if (g?.type === "Polygon") features.push({ type: "Feature", properties: props, geometry: { type: "MultiPolygon", coordinates: [g.coordinates] } });
    else if (g?.type === "MultiPolygon") features.push({ type: "Feature", properties: props, geometry: { type: "MultiPolygon", coordinates: g.coordinates } });
    else if (zones && zoneOnly(e)) {
      const coordinates = e.location!.zones!.flatMap((z) => zones.get(z) ?? []);
      if (coordinates.length) features.push({ type: "Feature", properties: props, geometry: { type: "MultiPolygon", coordinates } });
    }
  }
  return { type: "FeatureCollection" as const, features };
}

/** The events drawn as lines: tracks, such as a storm's path. */
export function toTracks(events: readonly Event[]) {
  const features: TrackFeature[] = [];
  for (const e of events) {
    const g = e.location?.geometry;
    if (g?.type === "LineString") features.push({ type: "Feature", properties: { id: e.id, type: e.type }, geometry: { type: "LineString", coordinates: g.coordinates } });
  }
  return { type: "FeatureCollection" as const, features };
}
