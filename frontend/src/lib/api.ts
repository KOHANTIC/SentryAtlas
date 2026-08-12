import { GeoJSONFeature, FetchParams, StreamSummary } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

function buildParams(params: FetchParams): URLSearchParams {
  const qs = new URLSearchParams();
  if (params.types.length > 0) qs.set("types", params.types.join(","));
  if (params.since) qs.set("since", params.since);
  if (params.bbox) qs.set("bbox", params.bbox.join(","));
  return qs;
}

export async function streamEvents(
  params: FetchParams,
  onChunk: (features: GeoJSONFeature[]) => void,
  signal: AbortSignal,
  onDone?: (summary: StreamSummary) => void
): Promise<void> {
  if (params.types.length === 0) return;

  const qs = buildParams(params);
  qs.set("format", "sse");

  const res = await fetch(`${API_URL}/api/v1/events?${qs.toString()}`, {
    signal,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new Error(body?.error ?? `API error: ${res.status}`);
  }

  if (!res.body) {
    throw new Error("API response has no body");
  }

  const reader = res.body.getReader();
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
        // Unlocated events (e.g. NOAA alerts without a polygon) arrive with
        // geometry: null and cannot be drawn on the map.
        const drawable = ((geojson.features ?? []) as { geometry: unknown }[])
          .filter((f) => f.geometry != null);
        if (drawable.length) {
          onChunk(drawable as GeoJSONFeature[]);
        }
      } else if (eventType === "done" && data) {
        onDone?.(JSON.parse(data) as StreamSummary);
      }
    }
  }
}
