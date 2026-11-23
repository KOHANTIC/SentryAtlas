"use client";

import { useMemo } from "react";
import MapView from "@/components/MapView";
import { filterEvents } from "@/lib/filters";
import { SOURCE_NAMES } from "@/lib/format";
import { useSnapshot } from "@/lib/useSnapshot";

const HOUR = 3_600_000;
const asOf = (iso: string) => {
  const d = new Date(iso);
  const day = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" }).format(d);
  const time = new Intl.DateTimeFormat("en-GB", { hour: "2-digit", minute: "2-digit", timeZone: "UTC" }).format(d);
  return `${day} · ${time} UTC`;
};

/** Today's figures and a live view of the map, from the same snapshot the map reads. */
export function Figures() {
  const { snapshot, checkedAt } = useSnapshot("7d");
  const figures = useMemo(() => {
    if (!snapshot || checkedAt === undefined) return undefined;
    const drawn = filterEvents(snapshot.events, {});
    return {
      day: filterEvents(drawn, { since: checkedAt - 24 * HOUR }).length,
      alerts: drawn.filter((e) => e.source === "noaa").length,
      quakes: drawn.filter((e) => e.type === "earthquake").length,
    };
  }, [snapshot, checkedAt]);
  const n = (x: number | undefined) => (x === undefined ? "–" : x.toLocaleString("en"));
  return (
    <div className="figures" aria-live="polite">
      <div><b>{n(figures?.day)}</b><span>events in the last 24 hours</span></div>
      <div><b>{n(figures?.alerts)}</b><span>US weather alerts in force</span></div>
      <div><b>{n(figures?.quakes)}</b><span>earthquakes in the last 7 days</span></div>
      <p className="label">{snapshot ? asOf(snapshot.generated) : "Loading"}</p>
    </div>
  );
}

/** A still, live view of the globe; "Open the map" above it is the way in. */
export function Thumbnail() {
  const { snapshot } = useSnapshot("7d");
  const events = useMemo(() => filterEvents(snapshot?.events ?? [], {}), [snapshot]);
  return (
    <div className="thumb">
      <MapView events={events} interactive={false} />
    </div>
  );
}

/** Each source and how many events it gives this week. */
export function SourceCounts() {
  const { snapshot } = useSnapshot("7d");
  return (
    <ul className="source-counts">
      {(snapshot?.sources ?? []).map((s) => (
        <li key={s.source}>
          <span>{SOURCE_NAMES[s.source]}</span>
          <b>{s.count.toLocaleString("en")}</b>
        </li>
      ))}
    </ul>
  );
}
