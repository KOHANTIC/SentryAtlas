import { EVENT_TYPES, GeoJSONFeature, FilterState } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

function buildParams(filters: FilterState): URLSearchParams {
  const params = new URLSearchParams();
  if (filters.types.length < EVENT_TYPES.length) {
    params.set("types", filters.types.join(","));
  }
  if (filters.since) {
    params.set("since", filters.since);
  }
  if (filters.bbox) {
    params.set("bbox", filters.bbox.join(","));
  }
  return params;
}

export async function streamEvents(
  filters: FilterState,
  onChunk: (features: GeoJSONFeature[]) => void,
  signal: AbortSignal
): Promise<void> {
  if (filters.types.length === 0) return;

  const params = buildParams(filters);
  params.set("format", "sse");

  const res = await fetch(`${API_URL}/api/v1/events?${params.toString()}`, {
    signal,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new Error(body?.error ?? `API error: ${res.status}`);
  }

  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split("\n\n");
    buffer = parts.pop()!;

    for (const part of parts) {
      let eventType = "";
      let data = "";
      for (const line of part.split("\n")) {
        if (line.startsWith("event: ")) eventType = line.slice(7);
        else if (line.startsWith("data: ")) data = line.slice(6);
      }
      if (eventType === "features" && data) {
        const geojson = JSON.parse(data);
        if (geojson.features?.length) {
          onChunk(geojson.features as GeoJSONFeature[]);
        }
      }
    }
  }
}
