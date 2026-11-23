"use client";

import { useCallback, useMemo, useState, useSyncExternalStore } from "react";
import EventList from "@/components/EventList";
import EventPanel from "@/components/EventPanel";
import Legend from "@/components/Legend";
import MapView, { type Camera } from "@/components/MapView";
import TimeControl, { spanLabel } from "@/components/TimeControl";
import type { Event } from "@/lib/api.gen";
import { filterEvents, freshness, position, PRESETS, type BBox } from "@/lib/filters";
import { sortEvents, type SortBy } from "@/lib/sort";
import { parse, pushedBySelection, pushView, replaceView, subscribeURL } from "@/lib/urlState";
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
        since: checkedAt === undefined ? undefined : checkedAt - (view.hours ? view.hours * 3_600_000 : preset.ms),
      }),
    [snapshot, checkedAt, view.types, view.hours, preset.ms],
  );
  const fresh = snapshot && checkedAt ? freshness(snapshot, checkedAt) : undefined;
  // The list holds what the map shows; with no map (no WebGL2) it holds everything.
  const [bbox, setBbox] = useState<BBox | undefined>();
  const [sort, setSort] = useState<SortBy>("time");
  const [tab, setTab] = useState<"list" | "key">("list");
  const inView = useMemo(() => sortEvents(bbox ? filterEvents(events, { bbox }) : events, sort), [events, bbox, sort]);
  const onMove = useCallback((c: Camera) => replaceView({ ...parse(window.location.search), ...c }), []);
  const [copied, setCopied] = useState<"" | "copied" | "failed">("");

  // Selection: the link's event, and the events under a click that hit several.
  const byId = useMemo(() => new Map<string, Event>((snapshot?.events ?? []).map((e) => [e.id, e])), [snapshot]);
  const [choices, setChoices] = useState<string[] | null>(null);
  const brief = view.event ? byId.get(view.event) : undefined;
  const at = brief ? position(brief) : undefined;
  const group = useMemo(
    () => (brief?.group_id ? (snapshot?.events ?? []).filter((e) => e.group_id === brief.group_id && e.id !== brief.id) : []),
    [brief, snapshot],
  );
  const select = useCallback((id: string) => {
    setChoices(null);
    // A history entry, so the back button closes the event.
    pushView({ ...parse(window.location.search), event: id });
  }, []);
  const onPick = useCallback((ids: string[]) => (ids.length === 1 ? select(ids[0]) : setChoices(ids)), [select]);
  const close = useCallback(() => {
    if (choices) return setChoices(null);
    if (pushedBySelection()) window.history.back();
    else replaceView({ ...parse(window.location.search), event: undefined });
  }, [choices]);

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setCopied("copied");
    } catch {
      setCopied("failed");
    }
  }

  return (
    <main id="main" className="map-page" data-panel={view.event || choices ? "open" : undefined}>
      <h1 className="sr-only">Map</h1>
      <MapView events={events} camera={view} onMove={onMove} selected={view.event ? { id: view.event, at } : undefined} onPick={onPick} onBounds={setBbox} />
      <EventPanel
        id={choices ? undefined : view.event}
        choices={choices?.map((id) => byId.get(id)).filter((e): e is Event => !!e)}
        brief={brief}
        group={group}
        onSelect={select}
        onClose={close}
      />
      <aside className="map-sheet">
        <p className="label" role="status">
          {state.status === "loading" && "Loading events"}
          {state.status === "error" && "The events could not be loaded; trying again"}
          {state.status === "ready" && `${inView.length.toLocaleString("en")} in view of ${events.length.toLocaleString("en")} events, last ${view.hours ? spanLabel(view.hours) : LABELS[view.time]}`}
        </p>
        <TimeControl
          time={view.time}
          hours={view.hours}
          onChange={(time, hours) => replaceView({ ...parse(window.location.search), time, hours })}
        />
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
        <div role="tablist" aria-label="Sheet" className="tabs">
          <button type="button" role="tab" id="tab-list" aria-controls="tabpanel-list" aria-selected={tab === "list"} onClick={() => setTab("list")}>List</button>
          <button type="button" role="tab" id="tab-key" aria-controls="tabpanel-key" aria-selected={tab === "key"} onClick={() => setTab("key")}>Key</button>
        </div>
        <div role="tabpanel" id="tabpanel-list" aria-labelledby="tab-list" hidden={tab !== "list"}>
          <EventList events={inView} selected={view.event} onSelect={select} sort={sort} onSort={setSort} />
        </div>
        <div role="tabpanel" id="tabpanel-key" aria-labelledby="tab-key" hidden={tab !== "key"}>
          <Legend />
        </div>
      </aside>
    </main>
  );
}
