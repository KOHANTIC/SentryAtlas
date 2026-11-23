"use client";

import { useCallback, useMemo, useState, useSyncExternalStore } from "react";
import Legend from "@/components/Legend";
import MapView, { type Camera } from "@/components/MapView";
import { filterEvents, freshness, PRESETS } from "@/lib/filters";
import { parse, replaceView, subscribeURL } from "@/lib/urlState";
import { useSnapshot } from "@/lib/useSnapshot";

const NAMES = { usgs: "USGS", eonet: "NASA EONET", noaa: "NWS", gdacs: "GDACS" } as const;
const LABELS = { "24h": "24 hours", "7d": "7 days", "30d": "30 days" } as const;

/** The map page: the globe, the key, and what the data's state is. */
export default function MapPage() {
  // The link is the state: the view, the filters and the selected event all live in it.
  const search = useSyncExternalStore(subscribeURL, () => window.location.search, () => "");
  const view = useMemo(() => parse(search), [search]);
  const preset = PRESETS[view.time];
  const state = useSnapshot(preset.window);
  const { snapshot, checkedAt } = state;
  const events = useMemo(
    () =>
      filterEvents(snapshot?.events ?? [], {
        types: view.types && new Set(view.types),
        since: checkedAt === undefined ? undefined : checkedAt - preset.ms,
      }),
    [snapshot, checkedAt, view.types, preset.ms],
  );
  const fresh = snapshot && checkedAt ? freshness(snapshot, checkedAt) : undefined;
  const onMove = useCallback((c: Camera) => replaceView({ ...parse(window.location.search), ...c }), []);
  const [copied, setCopied] = useState<"" | "copied" | "failed">("");

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setCopied("copied");
    } catch {
      setCopied("failed");
    }
  }

  return (
    <main id="main" className="map-page">
      <h1 className="sr-only">Map</h1>
      <MapView events={events} camera={view} onMove={onMove} />
      <aside className="map-sheet">
        <p className="label" role="status">
          {state.status === "loading" && "Loading events"}
          {state.status === "error" && "The events could not be loaded; trying again"}
          {state.status === "ready" && `${events.length.toLocaleString("en")} events, last ${LABELS[view.time]}`}
        </p>
        <div className="share">
          <button type="button" onClick={copyLink}>Copy link</button>
          <span role="status">
            {copied === "copied" && "Link copied"}
            {copied === "failed" && "Could not copy: the address bar holds the link"}
          </span>
        </div>
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
