// Mirrors the backend's canonical registry (backend/internal/models/types.go).
// "weather" is the NOAA fallback for alerts with no more specific class;
// "other" covers upstream categories no adapter maps yet.
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
  "weather",
  "other",
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

export interface SourceStatus {
  source: string;
  ok: boolean;
  error?: string;
}

// Payload of the SSE "done" event that terminates a stream.
export interface StreamSummary {
  total: number;
  sources: SourceStatus[];
}
