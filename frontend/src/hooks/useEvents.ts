"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { fetchEvents } from "@/lib/api";
import { EventsGeoJSON, FilterState } from "@/lib/types";

interface UseEventsResult {
  data: EventsGeoJSON | null;
  isLoading: boolean;
  error: string | null;
  refetch: () => void;
}

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

    try {
      const result = await fetchEvents(filters);
      if (!controller.signal.aborted) {
        setData(result);
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
  }, [filters]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  return { data, isLoading, error, refetch: load };
}
