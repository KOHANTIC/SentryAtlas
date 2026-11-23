"use client";

import "maplibre-gl/dist/maplibre-gl.css";
import { useEffect, useRef, useSyncExternalStore } from "react";
import type { Map as MapLibre, GeoJSONSource } from "maplibre-gl";
import tokens from "../../../brand/tokens.json";
import type { Event } from "@/lib/api.gen";
import { toFeatures } from "@/lib/features";
import { mapStyle } from "@/lib/mapStyle";
import { markImage, type Fill, type Shape } from "@/lib/marks";
import type { Theme } from "@/lib/tokens";

const DARK = "(prefers-color-scheme: dark)";

function subscribeTheme(cb: () => void) {
  const m = window.matchMedia(DARK);
  m.addEventListener("change", cb);
  return () => m.removeEventListener("change", cb);
}

const theme = (): Theme => (window.matchMedia(DARK).matches ? "dark" : "light");

/** MapLibre 6 needs WebGL2; checked once. */
let webgl2: boolean | undefined;
function hasWebGL2(): boolean {
  if (webgl2 === undefined) webgl2 = !!document.createElement("canvas").getContext("webgl2");
  return webgl2;
}
const noSubscribe = () => () => {};

export type Camera = { center: [number, number]; zoom: number; bearing: number };

type Props = {
  events: readonly Event[];
  /** The selected event and where it is drawn. */
  selected?: { id: string; at?: [number, number] };
  /** Called with the events under a click: one to open, or several to choose from. */
  onPick?: (ids: string[]) => void;
  /** Where to look, from the link; absent starts on the whole globe. */
  camera?: Partial<Camera>;
  /** Called when the person has moved the map. */
  onMove?: (c: Camera) => void;
};

/** The globe, with every placed event drawn as its type's mark and its severity's fill. */
export default function MapView({ events, camera, onMove, selected, onPick }: Props) {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<MapLibre | null>(null);
  const current = useRef(events);
  const moved = useRef(onMove);
  const picked = useRef(onPick);
  const chosen = useRef(selected);
  // The latest camera, read when the map is created: the first render, while hydrating,
  // sees no link yet, and the map library arrives after the link has been read.
  const start = useRef(camera);
  useEffect(() => {
    moved.current = onMove;
    picked.current = onPick;
    start.current = camera;
  }, [onMove, onPick, camera]);
  const t = useSyncExternalStore(subscribeTheme, theme, () => "light" as Theme);
  const canDraw = useSyncExternalStore(noSubscribe, hasWebGL2, () => true);

  // Create the map once, and give it the theme's style when the theme changes.
  useEffect(() => {
    if (!canDraw || !container.current) return;
    let cancelled = false;
    if (map.current) {
      map.current.setStyle(mapStyle(t));
      return;
    }
    void import("maplibre-gl").then(({ Map, setWorkerUrl }) => {
      if (cancelled || !container.current) return;
      // MapLibre builds its worker's address at run time, which the bundler cannot
      // rewrite, so the build copies the worker to the site's root (npm run gen).
      setWorkerUrl("/maplibre-gl-worker.mjs");
      // The globe's diameter is about 136 px at zoom 0: start it at 85% of the shorter side.
      const side = Math.min(container.current.clientWidth, container.current.clientHeight) || 600;
      const c = start.current;
      const m = new Map({
        container: container.current,
        style: mapStyle(t),
        center: c?.center ?? [10, 20],
        zoom: c?.zoom ?? Math.max(0.5, Math.log2((0.85 * side) / 136)),
        bearing: c?.bearing ?? 0,
        attributionControl: { compact: true },
      });
      // The view, readable by tests and tools: latitude, longitude, zoom.
      const mark = () => {
        const { lng, lat } = m.getCenter();
        container.current?.setAttribute("data-view", `${lat.toFixed(2)},${lng.toFixed(2)},${m.getZoom().toFixed(1)}`);
      };
      m.on("load", mark);
      m.on("moveend", mark);
      // Only moves the person makes reach the link; jumps from the link carry no originalEvent.
      m.on("moveend", (e) => {
        if (!("originalEvent" in e) || !e.originalEvent) return;
        const { lng, lat } = m.getCenter();
        moved.current?.({ center: [lng, lat], zoom: m.getZoom(), bearing: m.getBearing() });
      });
      map.current = m;
      // Marks are drawn when the map first asks for one: `<theme>:<type>:<fill>`.
      m.on("styleimagemissing", ({ id }) => {
        const [th, type, fill] = id.split(":") as [Theme, keyof typeof tokens.event, Fill];
        const e = tokens.event[type];
        if (!e) return;
        const ground = tokens.color[th]?.sheet ?? tokens.color.light.sheet;
        m.addImage(id, markImage(e.shape as Shape, fill, e[th], ground, 2), { pixelRatio: 2 });
      });
      m.on("style.load", () => {
        setData(m, current.current);
        setSelected(m, chosen.current);
      });
      const layers = ["marks", "quakes", "quake-clusters"];
      // A click takes every event within a few pixels, so stacked marks can each be reached.
      m.on("click", (e) => {
        const { x, y } = e.point;
        const hits = m.queryRenderedFeatures([[x - 6, y - 6], [x + 6, y + 6]], { layers });
        const cluster = hits.find((f) => f.properties?.cluster);
        if (cluster && cluster.geometry.type === "Point") {
          const at = cluster.geometry.coordinates as [number, number];
          void (m.getSource("quakes") as GeoJSONSource).getClusterExpansionZoom(cluster.properties.cluster_id).then((zoom) => m.easeTo({ center: at, zoom }));
          return;
        }
        const ids = [...new Set(hits.map((f) => String(f.properties?.id)).filter((id) => id !== "undefined"))];
        if (ids.length) picked.current?.(ids);
      });
      for (const l of layers) {
        m.on("mouseenter", l, () => (m.getCanvas().style.cursor = "pointer"));
        m.on("mouseleave", l, () => (m.getCanvas().style.cursor = ""));
      }
    });
    return () => {
      cancelled = true;
    };
  }, [canDraw, t]);

  useEffect(() => () => map.current?.remove(), []);

  // Back and forward change the link: follow it when it names a view the map is not showing.
  const [lon, lat] = camera?.center ?? [];
  const zoom = camera?.zoom;
  const bearing = camera?.bearing ?? 0;
  useEffect(() => {
    const m = map.current;
    if (!m || lon === undefined || lat === undefined || zoom === undefined) return;
    const c = m.getCenter();
    const far = Math.abs(c.lng - lon) > 1e-4 || Math.abs(c.lat - lat) > 1e-4 || Math.abs(m.getZoom() - zoom) > 0.01 || Math.abs(m.getBearing() - bearing) > 0.5;
    if (far) m.jumpTo({ center: [lon, lat], zoom, bearing });
  }, [lon, lat, zoom, bearing]);

  // Ring the selected event, and bring it into view if it is outside.
  const selId = selected?.id;
  const [sx, sy] = selected?.at ?? [];
  useEffect(() => {
    chosen.current = selId ? { id: selId, at: sx === undefined || sy === undefined ? undefined : [sx, sy] } : undefined;
    const m = map.current;
    if (!m) return;
    setSelected(m, chosen.current);
    if (sx !== undefined && sy !== undefined && !m.getBounds().contains([sx, sy])) {
      const options = { center: [sx, sy] as [number, number], zoom: Math.max(m.getZoom(), 4) };
      if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) m.jumpTo(options);
      else m.flyTo(options);
    }
  }, [selId, sx, sy]);

  useEffect(() => {
    current.current = events;
    // Before the style has its sources this does nothing; style.load then sets the latest events.
    if (map.current) setData(map.current, events);
  }, [events]);

  if (!canDraw) {
    return (
      <div className="map-fallback" role="status">
        <p>This browser cannot draw the map: it needs WebGL2.</p>
      </div>
    );
  }
  // MapLibre's stylesheet positions its own container, so the layout sits on a wrapper.
  return (
    <div className="map">
      <div ref={container} className="map-canvas" aria-label="Map of current events" role="region" />
    </div>
  );
}

function setSelected(m: MapLibre, s: Props["selected"]) {
  const data = s?.at
    ? { type: "FeatureCollection" as const, features: [{ type: "Feature" as const, properties: {}, geometry: { type: "Point" as const, coordinates: s.at } }] }
    : { type: "FeatureCollection" as const, features: [] };
  (m.getSource("selected") as GeoJSONSource | undefined)?.setData(data);
}

function setData(m: MapLibre, events: readonly Event[]) {
  const { quakes, marks } = toFeatures(events);
  (m.getSource("quakes") as GeoJSONSource | undefined)?.setData(quakes);
  (m.getSource("marks") as GeoJSONSource | undefined)?.setData(marks);
}
