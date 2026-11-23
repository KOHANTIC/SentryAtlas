"use client";

import { useEffect, useRef, useState } from "react";
import type { Event } from "@/lib/api.gen";
import { useDetail } from "@/lib/detail";
import { formatMagnitude, formatSeverity, formatTime, SOURCE_NAMES, typeLabel } from "@/lib/format";

type Props = {
  /** The selected event's id, from the link. */
  id?: string;
  /** The events under a click that hit several; each can be chosen. */
  choices?: Event[];
  /** The selected event as the snapshot has it, shown until its detail arrives. */
  brief?: Event;
  /** The other sources' reports of the same event. */
  group: Event[];
  onSelect: (id: string) => void;
  onClose: () => void;
};

type Height = "peek" | "half" | "full";
const TALLER: Record<Height, Height> = { peek: "half", half: "full", full: "full" };
const SHORTER: Record<Height, Height> = { peek: "peek", half: "peek", full: "half" };

/** Everything known about the selected event; on phones a bottom sheet worked by buttons. */
export default function EventPanel({ id, choices, brief, group, onSelect, onClose }: Props) {
  const detail = useDetail(id);
  const heading = useRef<HTMLHeadingElement>(null);
  const opener = useRef<Element | null>(null);
  const [height, setHeight] = useState<Height>("half");
  const open = Boolean(id || choices?.length);

  // Focus moves to the panel's heading when it opens or shows another event, and
  // back to where it came from when it closes.
  useEffect(() => {
    if (!open) return;
    if (!opener.current) opener.current = document.activeElement;
    heading.current?.focus();
  }, [open, id, choices]);

  if (!open) return null;

  const close = () => {
    const back = opener.current;
    opener.current = null;
    onClose();
    if (back instanceof HTMLElement && back.isConnected) back.focus();
  };

  const e = detail?.status === "ok" ? detail.event : brief;
  return (
    <section
      className="panel"
      data-height={height}
      aria-labelledby="panel-title"
      onKeyDown={(k) => k.key === "Escape" && close()}
    >
      <div className="panel-bar">
        <span className="sheet-steps">
          <button type="button" onClick={() => setHeight(TALLER[height])} disabled={height === "full"}>More</button>
          <button type="button" onClick={() => setHeight(SHORTER[height])} disabled={height === "peek"}>Less</button>
          <button type="button" onClick={() => setHeight("peek")}>Map</button>
        </span>
        <button type="button" className="panel-close" onClick={close} aria-label="Close">Close</button>
      </div>

      {choices?.length ? (
        <>
          <h2 id="panel-title" ref={heading} tabIndex={-1}>{choices.length} events here</h2>
          <ul className="choices">
            {choices.map((c) => (
              <li key={c.id}>
                <button type="button" onClick={() => onSelect(c.id)}>
                  <b>{c.title}</b>
                  <span>{typeLabel(c.type)} · {formatSeverity(c)} · {SOURCE_NAMES[c.source]}</span>
                </button>
              </li>
            ))}
          </ul>
        </>
      ) : detail?.status === "ended" && !brief ? (
        <>
          <h2 id="panel-title" ref={heading} tabIndex={-1}>This event has ended</h2>
          <p>It is no longer in its source&apos;s feed, so its details are gone.</p>
        </>
      ) : !e ? (
        <h2 id="panel-title" ref={heading} tabIndex={-1}>{detail?.status === "error" ? "The event could not be loaded" : "Loading the event"}</h2>
      ) : (
        <>
          <span className="label">{typeLabel(e.type)} · {SOURCE_NAMES[e.source]}</span>
          <h2 id="panel-title" ref={heading} tabIndex={-1}>{e.title}</h2>
          <p className="sevword">{formatSeverity(e)}</p>
          <dl className="facts">
            {formatMagnitude(e) && (<><dt>Magnitude</dt><dd>{formatMagnitude(e)}</dd></>)}
            {e.details.usgs?.depth_km !== undefined && (<><dt>Depth</dt><dd>{e.details.usgs.depth_km.toFixed(1)} km</dd></>)}
            {place(e) && (<><dt>Where</dt><dd>{place(e)}</dd></>)}
            <dt>Began</dt><dd>{formatTime(e.time.started)}</dd>
            <dt>Updated</dt><dd>{formatTime(e.time.updated)}</dd>
            {e.time.ends && (<><dt>Ends</dt><dd>{formatTime(e.time.ends)}</dd></>)}
            {e.time.expires && (<><dt>Expires</dt><dd>{formatTime(e.time.expires)}</dd></>)}
          </dl>
          {e.details.noaa?.instruction && (
            <div className="do">
              <b>What to do</b>
              <p className="text">{e.details.noaa.instruction}</p>
            </div>
          )}
          {e.description && <p className="text">{e.description}</p>}
          {detail?.status === "loading" && <p className="muted">Loading the source&apos;s text</p>}
          {detail?.status === "error" && <p className="muted">The source&apos;s text could not be loaded.</p>}
          {detail?.status === "ended" && <p className="muted">This event has left its source&apos;s feed; its full text is gone.</p>}
          {e.url && (
            <p>
              <a href={e.url} target="_blank" rel="noopener noreferrer">Open it on {SOURCE_NAMES[e.source]}</a>
            </p>
          )}
          {group.length > 0 && (
            <div className="group">
              <span className="label">Also reported by</span>
              <ul>
                {group.map((g) => (
                  <li key={g.id}>
                    <button type="button" onClick={() => onSelect(g.id)}>{SOURCE_NAMES[g.source]}: {g.title}</button>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </>
      )}
    </section>
  );
}

/** Where the source says the event is, in its own words. */
function place(e: Event): string | undefined {
  return e.details.usgs?.place ?? e.details.noaa?.area ?? e.details.gdacs?.countries?.join(", ");
}
