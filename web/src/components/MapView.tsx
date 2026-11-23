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

/** The globe, with every placed event drawn as its type's mark and its severity's fill. */
export default function MapView({ events }: { events: readonly Event[] }) {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<MapLibre | null>(null);
  const current = useRef(events);
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
      const m = new Map({ container: container.current, style: mapStyle(t), center: [0, 20], zoom: 1.4, attributionControl: { compact: true } });
      map.current = m;
      // Marks are drawn when the map first asks for one: `<theme>:<type>:<fill>`.
      m.on("styleimagemissing", ({ id }) => {
        const [th, type, fill] = id.split(":") as [Theme, keyof typeof tokens.event, Fill];
        const e = tokens.event[type];
        if (!e) return;
        const ground = tokens.color[th]?.sheet ?? tokens.color.light.sheet;
        m.addImage(id, markImage(e.shape as Shape, fill, e[th], ground, 2), { pixelRatio: 2 });
      });
      m.on("style.load", () => setData(m, current.current));
    });
    return () => {
      cancelled = true;
    };
  }, [canDraw, t]);

  useEffect(() => () => map.current?.remove(), []);

  useEffect(() => {
    current.current = events;
    if (map.current?.isStyleLoaded()) setData(map.current, events);
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

function setData(m: MapLibre, events: readonly Event[]) {
  const { quakes, marks } = toFeatures(events);
  (m.getSource("quakes") as GeoJSONSource | undefined)?.setData(quakes);
  (m.getSource("marks") as GeoJSONSource | undefined)?.setData(marks);
}
