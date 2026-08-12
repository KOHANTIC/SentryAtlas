"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { streamEvents } from "@/lib/api";
import type {
  EventsGeoJSON,
  GeoJSONFeature,
  FetchParams,
  SourceStatus,
} from "@/lib/types";

// The backend caches upstream responses for ~5 minutes; refreshing faster
// than that only re-reads the same cache.
const REFRESH_INTERVAL_MS = 5 * 60 * 1000;

interface UseEventsResult {
  data: EventsGeoJSON | null;
  isLoading: boolean;
  error: string | null;
  /** Per-upstream status from the last completed stream. */
  sources: SourceStatus[] | null;
  /** When the last stream completed successfully. */
  lastUpdated: Date | null;
  refetch: () => void;
}

const EMPTY_GEOJSON: EventsGeoJSON = { type: "FeatureCollection", features: [] };

export function useEvents(params: FetchParams): UseEventsResult {
  const [data, setData] = useState<EventsGeoJSON | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sources, setSources] = useState<SourceStatus[] | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
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

    // The previous data stays on the map until the new stream delivers its
    // first batch — clearing eagerly made every pan/filter flash a blank map.
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
        controller.signal,
        (summary) => {
          if (controller.signal.aborted) return;
          setSources(summary.sources);
        }
      );
      if (!controller.signal.aborted) {
        // Commit the final result even when the stream produced nothing,
        // so stale data doesn't linger after an empty query.
        setData({
          type: "FeatureCollection",
          features: Array.from(accumulated.values()),
        });
        setLastUpdated(new Date());
      }
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

  // Live data goes stale quietly: refresh on an interval so "real-time
  // monitoring" stays true for a tab left open.
  useEffect(() => {
    const id = setInterval(load, REFRESH_INTERVAL_MS);
    return () => clearInterval(id);
  }, [load]);

  return { data, isLoading, error, sources, lastUpdated, refetch: load };
}
