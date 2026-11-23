"use client";

import { useState, useCallback, useRef } from "react";
import dynamic from "next/dynamic";
import { useEvents } from "@/hooks/useEvents";
import FilterPanel from "@/components/FilterPanel";
import Legend from "@/components/Legend";
import { EVENT_TYPES, type FetchParams, type EventType } from "@/lib/types";
import { getSinceDate } from "@/lib/time";

const MapView = dynamic(() => import("@/components/MapView"), { ssr: false });

const SOURCE_NAMES: Record<string, string> = {
  noaa: "NWS",
  usgs: "USGS",
  gdacs: "GDACS",
  eonet: "NASA EONET",
};

// How long a source has gone without a good answer, from when the copy the
// map still shows was fetched.
function silentFor(fetchedAt: string): string {
  const mins = Math.max(1, Math.round((Date.now() - Date.parse(fetchedAt)) / 60_000));
  if (mins < 60) return mins === 1 ? "1 minute" : `${mins} minutes`;
  const hours = Math.round(mins / 60);
  return hours === 1 ? "1 hour" : `${hours} hours`;
}

function relativeAge(from: Date): string {
  const mins = Math.round((Date.now() - from.getTime()) / 60_000);
  if (mins < 1) return "just now";
  if (mins === 1) return "1 min ago";
  return `${mins} min ago`;
}

export default function Home() {
  // All types visible by default: an empty selection renders a blank world
  // map with "0 events", which reads as broken to a first-time visitor.
  const [fetchParams, setFetchParams] = useState<FetchParams>({
    types: [...EVENT_TYPES],
    since: getSinceDate("7d"),
  });
  const { data, isLoading, error, sources, lastUpdated, refetch } =
    useEvents(fetchParams);

  const boundsTimer = useRef<ReturnType<typeof setTimeout>>(null);
  const handleBoundsChange = useCallback(
    (bbox: [number, number, number, number]) => {
      if (boundsTimer.current) clearTimeout(boundsTimer.current);
      boundsTimer.current = setTimeout(() => {
        setFetchParams((prev) => ({ ...prev, bbox }));
      }, 300);
    },
    []
  );

  const handleTypesChange = useCallback((types: EventType[]) => {
    setFetchParams((prev) => ({ ...prev, types }));
  }, []);

  const handleSinceChange = useCallback((since?: string) => {
    setFetchParams((prev) => ({ ...prev, since }));
  }, []);

  const eventCount = data?.features?.length ?? 0;
  const downSources = (sources ?? []).filter((s) => !s.ok);
  const nothingSelected = fetchParams.types.length === 0;

  return (
    <main className="relative h-dvh w-full overflow-hidden">
      <h1 className="sr-only">SentryAtlas — real-time disaster monitoring</h1>
      <MapView data={data} onBoundsChange={handleBoundsChange} />

      {/* One column for both left-hand panels: absolutely positioning them
          independently let the expanded legend overlap the filter panel and
          run off short viewports. */}
      <div className="absolute top-4 bottom-6 left-4 z-10 flex flex-col items-start justify-between gap-3 max-w-[calc(100vw-6.5rem)] pointer-events-none">
        <div className="min-h-0 flex-shrink pointer-events-auto">
          <FilterPanel
            visibleTypes={fetchParams.types}
            onVisibleTypesChange={handleTypesChange}
            since={fetchParams.since}
            onSinceChange={handleSinceChange}
          />
        </div>

        <div className="min-h-0 flex-shrink-0 pointer-events-auto">
          <Legend />
        </div>
      </div>

      <div className="absolute top-4 right-4 z-10 flex flex-col items-end gap-2">
        <div className="bg-surface-raised/95 backdrop-blur-sm border border-border shadow-lg px-4 py-2 flex items-center gap-3">
          {isLoading && (
            <div
              className="flex items-center gap-2 text-xs text-foreground-muted"
              role="status"
            >
              <svg
                className="motion-safe:animate-spin h-3.5 w-3.5"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                aria-hidden="true"
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
              <span className="whitespace-nowrap">
                {eventCount > 0
                  ? `Loading… (${eventCount} loaded)`
                  : "Loading events…"}
              </span>
            </div>
          )}
          {error && (
            <div
              className="flex items-center gap-2 text-xs font-medium text-error"
              role="alert"
            >
              <span className="max-w-[40vw] truncate" title={error}>
                {error}
              </span>
              <button
                onClick={refetch}
                className="underline underline-offset-2 hover:no-underline cursor-pointer whitespace-nowrap"
              >
                Retry
              </button>
            </div>
          )}
          {!isLoading && !error && (
            <span className="text-xs text-foreground-muted font-medium whitespace-nowrap">
              {eventCount} {eventCount === 1 ? "event" : "events"}
              {lastUpdated && (
                <span className="text-foreground-faint">
                  {" "}
                  · {relativeAge(lastUpdated)}
                </span>
              )}
            </span>
          )}
        </div>

        {!isLoading && !error && nothingSelected && (
          <div className="bg-surface-raised/95 backdrop-blur-sm border border-border shadow-lg px-3 py-2 text-xs text-foreground-muted max-w-56">
            No event types selected — choose some in Filters, or{" "}
            <button
              onClick={() => handleTypesChange([...EVENT_TYPES])}
              className="font-semibold text-accent-400 hover:underline cursor-pointer"
            >
              show all
            </button>
            .
          </div>
        )}

        {!isLoading && !error && !nothingSelected && eventCount === 0 && (
          <div className="bg-surface-raised/95 backdrop-blur-sm border border-border shadow-lg px-3 py-2 text-xs text-foreground-muted max-w-56">
            No events match the current filters in this view. Try a wider time
            range or zoom out.
          </div>
        )}

        {downSources.length > 0 && (
          <div
            className="bg-surface-raised/95 backdrop-blur-sm border border-border shadow-lg px-3 py-2 text-xs text-foreground-muted max-w-56"
            role="status"
            title={downSources
              .map((s) => `${SOURCE_NAMES[s.source] ?? s.source}: ${s.error ?? "unavailable"}`)
              .join("\n")}
          >
            {downSources.map((s) => (
              <p key={s.source}>
                <span className="font-semibold">{SOURCE_NAMES[s.source] ?? s.source}</span>{" "}
                {/* Since #22 a source whose poll failed stays on the map
                    from its last copy; only one with none is unavailable. */}
                {s.stale && s.fetched_at
                  ? `has not answered for ${silentFor(s.fetched_at)}; showing its last update.`
                  : "is currently unavailable."}
              </p>
            ))}
          </div>
        )}
      </div>

      {/* On phones the map's attribution bar spans the bottom edge, so the
          link sits above it; from sm up the attribution stays at the right. */}
      <div className="absolute bottom-11 sm:bottom-2 left-1/2 -translate-x-1/2 z-10">
        <span className="text-[11px] text-foreground-muted bg-surface-overlay border border-border px-2 py-1 whitespace-nowrap">
          <span className="hidden sm:inline">
            Made with ♥ by the{" "}
            <a href="https://kohantic.com" target="_blank" rel="noopener noreferrer" className="font-bold text-accent-400 hover:underline">KOHANTIC</a>{" "}
            team ·{" "}
          </span>
          {/* AGPL §13: a network-facing deployment must offer users its source,
              so the link shows at every width, the credit only where it fits. */}
          <a
            href="https://github.com/KOHANTIC/SentryAtlas"
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-foreground hover:underline"
          >
            Source
          </a>
          {" · "}
          <a
            href="https://sentryatlas.com/privacy/"
            className="hover:text-foreground hover:underline"
          >
            Privacy
          </a>
        </span>
      </div>
    </main>
  );
}
