"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { streamEvents } from "@/lib/api";
import { EventsGeoJSON, GeoJSONFeature, FilterState } from "@/lib/types";

interface UseEventsResult {
  data: EventsGeoJSON | null;
  isLoading: boolean;
  error: string | null;
  refetch: () => void;
}

const EMPTY_GEOJSON: EventsGeoJSON = { type: "FeatureCollection", features: [] };

export function useEvents(filters: FilterState): UseEventsResult {
  const [data, setData] = useState<EventsGeoJSON | null>(null);
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

    if (filters.types.length === 0) {
      setIsLoading(false);
      return;
    }

    const accumulated = new Map<string, GeoJSONFeature>();

    try {
      await streamEvents(
        filters,
        (features) => {
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
  }, [filters]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  return { data, isLoading, error, refetch: load };
}
