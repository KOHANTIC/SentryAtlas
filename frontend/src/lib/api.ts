import { EVENT_TYPES, EventsGeoJSON, FilterState } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

const EMPTY_GEOJSON: EventsGeoJSON = { type: "FeatureCollection", features: [] };

export async function fetchEvents(filters: FilterState): Promise<EventsGeoJSON> {
  if (filters.types.length === 0) {
    return EMPTY_GEOJSON;
  }

  const params = new URLSearchParams();
  params.set("format", "geojson");

  if (filters.types.length < EVENT_TYPES.length) {
    params.set("types", filters.types.join(","));
  }

  if (filters.since) {
    params.set("since", filters.since);
  }

  if (filters.bbox) {
    params.set("bbox", filters.bbox.join(","));
  }

  const res = await fetch(`${API_URL}/api/v1/events?${params.toString()}`);

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new Error(body?.error ?? `API error: ${res.status}`);
  }

  return res.json();
}
