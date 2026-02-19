import { GeoJSONFeature, FetchParams } from "./types";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

function buildParams(params: FetchParams): URLSearchParams {
  const qs = new URLSearchParams();
  if (params.since) qs.set("since", params.since);
  if (params.bbox) qs.set("bbox", params.bbox.join(","));
  return qs;
}

export async function streamEvents(
  params: FetchParams,
  onChunk: (features: GeoJSONFeature[]) => void,
  signal: AbortSignal
): Promise<void> {
  const qs = buildParams(params);
  qs.set("format", "sse");

  const res = await fetch(`${API_URL}/api/v1/events?${qs.toString()}`, {
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
