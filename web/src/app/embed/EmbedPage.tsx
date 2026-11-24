"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import MapView, { type Camera } from "@/components/MapView";
import { Logo } from "@/components/Site";
import type { Event } from "@/lib/api.gen";
import { position } from "@/lib/filters";
import { formatMagnitude, formatShort, SOURCE_NAMES, sourceNotes, typeLabel, windowLabel } from "@/lib/format";
import { parse, replaceView, serialize } from "@/lib/urlState";
import { useLinkedEvents } from "@/lib/useLinkedEvents";

/** The map on another site's page: the link's view and filters, a credit bar, and links back to the full map. */
export default function EmbedPage() {
  const { view, state, events, fresh } = useLinkedEvents();
  // The reader's moves go into the frame's own link, so "Open the full map" opens where they are.
  const onMove = useCallback((c: Camera) => replaceView({ ...parse(window.location.search), ...c }), []);
  const byId = useMemo(() => new Map(events.map((e) => [e.id, e])), [events]);
  const [picked, setPicked] = useState<Event[] | null>(null);
  const onPick = useCallback((ids: string[]) => setPicked(ids.map((id) => byId.get(id)).filter((e): e is Event => !!e)), [byId]);
  const one = picked?.length === 1 ? picked[0] : undefined;
  const full = (event?: string) => `/map/${serialize({ ...view, event })}`;

  return (
    <main id="main" className="embed-page">
      <h1 className="sr-only">SentryAtlas map</h1>
      <div className="embed-map">
        <MapView
          events={events}
          camera={view}
          onMove={onMove}
          onPick={onPick}
          selected={one ? { id: one.id, at: position(one) } : undefined}
          cooperative
          layers={view.layers}
          fallback="This browser cannot draw the map: it needs WebGL2."
        />
        {picked?.length ? <PickCard events={picked} link={full} onClose={() => setPicked(null)} /> : null}
      </div>
      <footer className="embed-bar">
        <a className="embed-credit" href={full()} target="_blank" rel="noopener">
          <Logo size={18} />
          <b>SentryAtlas</b>
          <span>Open the full map</span>
          <span className="sr-only"> (opens in a new tab)</span>
        </a>
        <span role="status">
          {state.status === "loading" && "Loading events"}
          {state.status === "error" && "The events could not be loaded; trying again"}
          {state.status === "ready" && `${events.length.toLocaleString("en")} events, last ${windowLabel(view.time, view.hours)}`}
        </span>
        {fresh?.partial && (
          <ul className="notice">
            {sourceNotes(fresh.sources).map((n) => (
              <li key={n.name}>
                <b>{n.name}</b> {n.note}
              </li>
            ))}
          </ul>
        )}
      </footer>
    </main>
  );
}

/** The events under a click, each a link to the full map open on it. */
function PickCard({ events, link, onClose }: { events: Event[]; link: (event: string) => string; onClose: () => void }) {
  const heading = useRef<HTMLHeadingElement>(null);
  const opener = useRef<Element | null>(null);
  const closeRef = useRef(onClose);
  useEffect(() => {
    closeRef.current = onClose;
  });

  // Focus moves to the card when it opens or shows other events, and back when it closes.
  useEffect(() => {
    if (!opener.current) opener.current = document.activeElement;
    heading.current?.focus();
  }, [events]);
  useEffect(() => {
    const back = opener;
    const onKey = (k: KeyboardEvent) => k.key === "Escape" && closeRef.current();
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      if (back.current instanceof HTMLElement && back.current.isConnected) back.current.focus();
    };
  }, []);

  return (
    <section className="embed-card" aria-labelledby="embed-card-title">
      <div className="embed-card-bar">
        <h2 id="embed-card-title" ref={heading} tabIndex={-1}>
          {events.length === 1 ? "1 event here" : `${events.length} events here`}
        </h2>
        <button type="button" onClick={onClose}>Close</button>
      </div>
      <ul>
        {events.map((e) => (
          <li key={e.id}>
            <a href={link(e.id)} target="_blank" rel="noopener">
              {e.title}
              <span className="sr-only"> (opens in a new tab)</span>
            </a>
            <span>
              {formatMagnitude(e) ?? typeLabel(e.type)} · {SOURCE_NAMES[e.source]} · {formatShort(e.time.updated)}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
