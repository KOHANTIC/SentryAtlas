"use client";

import { useMemo } from "react";
import Legend from "@/components/Legend";
import MapView from "@/components/MapView";
import { filterEvents, freshness } from "@/lib/filters";
import { useSnapshot } from "@/lib/useSnapshot";

const NAMES = { usgs: "USGS", eonet: "NASA EONET", noaa: "NWS", gdacs: "GDACS" } as const;

/** The map page: the globe, the key, and what the data's state is. */
export default function MapPage() {
  const state = useSnapshot("7d");
  const events = useMemo(() => filterEvents(state.snapshot?.events ?? [], {}), [state.snapshot]);
  const fresh = state.snapshot && state.checkedAt ? freshness(state.snapshot, state.checkedAt) : undefined;

  return (
    <main id="main" className="map-page">
      <h1 className="sr-only">Map</h1>
      <MapView events={events} />
      <aside className="map-sheet">
        <p className="label" role="status">
          {state.status === "loading" && "Loading events"}
          {state.status === "error" && "The events could not be loaded; trying again"}
          {state.status === "ready" && `${events.length.toLocaleString("en")} events, last 7 days`}
        </p>
        {fresh?.partial && (
          <ul className="notice">
            {fresh.sources
              .filter((s) => !s.ok || s.stale)
              .map((s) => (
                <li key={s.source}>
                  <b>{NAMES[s.source]}</b>{" "}
                  {s.stale && s.silentMinutes !== undefined
                    ? `has not answered for ${s.silentMinutes} minutes; showing its last update.`
                    : "is unavailable."}
                </li>
              ))}
          </ul>
        )}
        <Legend />
      </aside>
    </main>
  );
}
