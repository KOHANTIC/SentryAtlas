export const EVENT_TYPES = [
  "earthquake",
  "wildfire",
  "volcano",
  "storm",
  "flood",
  "cyclone",
  "tornado",
  "hurricane",
  "winter_storm",
  "tsunami",
  "drought",
  "iceberg",
  "landslide",
] as const;

export type EventType = (typeof EVENT_TYPES)[number];

export type Severity = "extreme" | "severe" | "moderate" | "minor";

export interface EventProperties {
  id: string;
  title: string;
  event_type: EventType;
  source: string;
  severity?: Severity;
  magnitude?: number;
  started_at: string;
  updated_at: string;
  url?: string;
  description?: string;
  metadata?: Record<string, unknown>;
}

export interface GeoJSONFeature {
  type: "Feature";
  geometry: {
    type: "Point";
    coordinates: [number, number] | [number, number, number];
  };
  properties: EventProperties;
}

export interface EventsGeoJSON {
  type: "FeatureCollection";
  features: GeoJSONFeature[];
}

export interface FetchParams {
  types: EventType[];
  since?: string;
  bbox?: [number, number, number, number];
}
