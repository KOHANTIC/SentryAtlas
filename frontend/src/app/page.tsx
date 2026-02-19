"use client";

import { useState, useCallback, useRef, useEffect, useMemo } from "react";
import dynamic from "next/dynamic";
import { useEvents } from "@/hooks/useEvents";
import FilterPanel from "@/components/FilterPanel";
import Legend from "@/components/Legend";
import type { FetchParams, EventType } from "@/lib/types";
import { getSinceDate } from "@/lib/time";

const MapView = dynamic(() => import("@/components/MapView"), { ssr: false });

export default function Home() {
  const [fetchParams, setFetchParams] = useState<FetchParams>({
    since: getSinceDate("7d"),
  });
  const [visibleTypes, setVisibleTypes] = useState<EventType[]>([]);
  const boundsTimerRef = useRef<ReturnType<typeof setTimeout>>(null);

  const { data, loadedTypes, isLoading, error } = useEvents(fetchParams);

  useEffect(() => {
    if (loadedTypes.length === 0) return;
    setVisibleTypes((prev) => {
      const combined = new Set([...prev, ...loadedTypes]);
      return combined.size !== prev.length ? [...combined] : prev;
    });
  }, [loadedTypes]);

  const displayData = useMemo(() => {
    if (!data?.features.length) return data;
    const typeSet = new Set(visibleTypes);
    return {
      type: "FeatureCollection" as const,
      features: data.features.filter((f) =>
        typeSet.has(f.properties.event_type)
      ),
    };
  }, [data, visibleTypes]);

  const handleBoundsChange = useCallback(
    (bbox: [number, number, number, number]) => {
      if (boundsTimerRef.current) clearTimeout(boundsTimerRef.current);
      boundsTimerRef.current = setTimeout(() => {
        setFetchParams((prev) => ({ ...prev, bbox }));
      }, 2000);
    },
    []
  );

  const handleSinceChange = useCallback((since?: string) => {
    setVisibleTypes([]);
    setFetchParams((prev) => ({ ...prev, since }));
  }, []);

  const eventCount = displayData?.features?.length ?? 0;
  const totalCount = data?.features?.length ?? 0;

  return (
    <div className="relative h-screen w-screen overflow-hidden">
      <MapView data={displayData} onBoundsChange={handleBoundsChange} />

      <div className="absolute top-4 left-4 z-10">
        <FilterPanel
          visibleTypes={visibleTypes}
          onVisibleTypesChange={setVisibleTypes}
          since={fetchParams.since}
          onSinceChange={handleSinceChange}
        />
      </div>

      <div className="absolute bottom-6 left-4 z-10">
        <Legend />
      </div>

      <div className="absolute bottom-6 left-1/2 -translate-x-1/2 z-10">
        <div className="bg-white/90 backdrop-blur-sm rounded-full shadow-lg px-4 py-2 flex items-center gap-3">
          {isLoading && (
            <div className="flex items-center gap-2 text-xs text-gray-500">
              <svg
                className="animate-spin h-3.5 w-3.5"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  className="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="4"
                />
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                />
              </svg>
              {totalCount > 0
                ? `Loading events... (${totalCount} loaded)`
                : "Loading events..."}
            </div>
          )}
          {error && (
            <span className="text-xs text-red-600 font-medium">{error}</span>
          )}
          {!isLoading && !error && (
            <span className="text-xs text-gray-600 font-medium">
              {eventCount} {eventCount === 1 ? "event" : "events"}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
