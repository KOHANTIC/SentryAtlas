"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { streamEvents } from "@/lib/api";
import type { EventsGeoJSON, GeoJSONFeature, FetchParams } from "@/lib/types";

interface UseEventsResult {
  data: EventsGeoJSON | null;
  isLoading: boolean;
  error: string | null;
}

const EMPTY_GEOJSON: EventsGeoJSON = { type: "FeatureCollection", features: [] };

export function useEvents(params: FetchParams): UseEventsResult {
  const [data, setData] = useState<EventsGeoJSON | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const load = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setError(null);

    if (params.types.length === 0) {
      setData(EMPTY_GEOJSON);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    setData(EMPTY_GEOJSON);

    const accumulated = new Map<string, GeoJSONFeature>();

    try {
      await streamEvents(
        params,
        (features) => {
          if (controller.signal.aborted) return;
          for (const f of features) {
            accumulated.set(f.properties.id, f);
          }
          setData({
            type: "FeatureCollection",
            features: Array.from(accumulated.values()),
          });
        },
        controller.signal
      );
    } catch (err) {
      if (!controller.signal.aborted) {
        setError(err instanceof Error ? err.message : "Failed to fetch events");
      }
    } finally {
      if (!controller.signal.aborted) {
        setIsLoading(false);
      }
    }
  }, [params]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  return { data, isLoading, error };
}
