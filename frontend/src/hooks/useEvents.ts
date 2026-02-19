"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { streamEvents } from "@/lib/api";
import type { EventsGeoJSON, GeoJSONFeature, FetchParams, EventType } from "@/lib/types";

interface UseEventsResult {
  data: EventsGeoJSON | null;
  loadedTypes: EventType[];
  isLoading: boolean;
  error: string | null;
}

const EMPTY_GEOJSON: EventsGeoJSON = { type: "FeatureCollection", features: [] };

export function useEvents(params: FetchParams): UseEventsResult {
  const [data, setData] = useState<EventsGeoJSON | null>(null);
  const [loadedTypes, setLoadedTypes] = useState<EventType[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const load = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setIsLoading(true);
    setError(null);
    setData(EMPTY_GEOJSON);
    setLoadedTypes([]);

    const accumulated = new Map<string, GeoJSONFeature>();
    const discoveredTypes = new Set<EventType>();

    try {
      await streamEvents(
        params,
        (features) => {
          if (controller.signal.aborted) return;

          let hasNewTypes = false;
          for (const f of features) {
            accumulated.set(f.properties.id, f);
            if (!discoveredTypes.has(f.properties.event_type)) {
              discoveredTypes.add(f.properties.event_type);
              hasNewTypes = true;
            }
          }

          setData({
            type: "FeatureCollection",
            features: Array.from(accumulated.values()),
          });

          if (hasNewTypes) {
            setLoadedTypes(Array.from(discoveredTypes));
          }
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

  return { data, loadedTypes, isLoading, error };
}
