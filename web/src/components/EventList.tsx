"use client";

import { useEffect, useRef, useState } from "react";
import type { Event } from "@/lib/api.gen";
import { position } from "@/lib/filters";
import { formatMagnitude, formatShort, SOURCE_NAMES, typeLabel } from "@/lib/format";
import { distanceKm, formatKm } from "@/lib/near";
import type { SortBy } from "@/lib/sort";
import tokens from "../../../brand/tokens.json";
import Mark, { eventMark } from "./Mark";

const ROW = 56;
const SPARE = 6;

type Props = {
  events: readonly Event[];
  selected?: string;
  onSelect: (id: string) => void;
  sort: SortBy;
  onSort: (s: SortBy) => void;
  /** The viewer's position, when they used Near me: rows then say how far. */
  from?: [number, number];
};

/**
 * The events in view as a list, tied to the map both ways. It is a listbox:
 * arrow keys, Page Up and Down, Home and End move, Enter opens. Only the rows
 * in sight are drawn, so thousands stay smooth.
 */
export default function EventList({ events, selected, onSelect, sort, onSort, from }: Props) {
  const box = useRef<HTMLDivElement>(null);
  const [scroll, setScroll] = useState(0);
  const [height, setHeight] = useState(ROW * 8);
  const [active, setActive] = useState(0);

  // A selection made on the map moves the list to it.
  const [seen, setSeen] = useState(selected);
  if (selected !== seen) {
    setSeen(selected);
    const i = events.findIndex((e) => e.id === selected);
    if (i >= 0) setActive(i);
  }
  const act = Math.max(0, Math.min(active, events.length - 1));

  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setHeight(el.clientHeight));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Keep the active row in sight.
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const top = act * ROW;
    if (top < el.scrollTop) el.scrollTop = top;
    else if (top + ROW > el.scrollTop + el.clientHeight) el.scrollTop = top + ROW - el.clientHeight;
  }, [act]);

  const first = Math.max(0, Math.floor(scroll / ROW) - SPARE);
  const last = Math.min(events.length, Math.ceil((scroll + height) / ROW) + SPARE);
  const page = Math.max(1, Math.floor(height / ROW) - 1);

  function key(k: React.KeyboardEvent) {
    const to = { ArrowDown: act + 1, ArrowUp: act - 1, PageDown: act + page, PageUp: act - page, Home: 0, End: events.length - 1 }[k.key];
    if (to !== undefined) {
      k.preventDefault();
      setActive(Math.max(0, Math.min(to, events.length - 1)));
    } else if ((k.key === "Enter" || k.key === " ") && events[act]) {
      k.preventDefault();
      onSelect(events[act].id);
    }
  }

  return (
    <div className="list">
      <div className="list-sort" role="group" aria-label="Sort">
        <button type="button" aria-pressed={sort === "time"} onClick={() => onSort("time")}>Newest</button>
        <button type="button" aria-pressed={sort === "severity"} onClick={() => onSort("severity")}>Most severe</button>
        {from && <button type="button" aria-pressed={sort === "distance"} onClick={() => onSort("distance")}>Nearest</button>}
      </div>
      {events.length === 0 ? (
        <p className="muted">No events here. Zoom out, or widen the time window.</p>
      ) : (
        <div
          ref={box}
          className="list-box"
          role="listbox"
          aria-label="Events in view"
          tabIndex={0}
          aria-activedescendant={`ev-${act}`}
          onKeyDown={key}
          onScroll={(e) => setScroll(e.currentTarget.scrollTop)}
        >
          <div style={{ height: events.length * ROW, position: "relative" }}>
            {events.slice(first, last).map((e, n) => {
              const i = first + n;
              const m = eventMark(e.type, e.severity?.level);
              const p = from && position(e);
              const away = from && p ? `${formatKm(distanceKm(from, p))} · ` : "";
              return (
                <div
                  key={e.id}
                  id={`ev-${i}`}
                  role="option"
                  aria-selected={e.id === selected}
                  aria-setsize={events.length}
                  aria-posinset={i + 1}
                  className="row"
                  data-active={i === act || undefined}
                  style={{ top: i * ROW, height: ROW }}
                  onClick={() => {
                    setActive(i);
                    onSelect(e.id);
                  }}
                >
                  <Mark {...m} />
                  <span className="row-title">{e.title}</span>
                  <span className="row-meta">
                    {away}
                    {(e.severity ? tokens.severity[e.severity.level].word : tokens.severity.ungraded.word) + " · "}
                    {(formatMagnitude(e) ?? typeLabel(e.type)) + " · "}
                    {SOURCE_NAMES[e.source]} · {formatShort(e.time.updated)}
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
