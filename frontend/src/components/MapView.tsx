"use client";

import { useRef, useEffect, useCallback } from "react";
import maplibregl from "maplibre-gl";
import {
  HEATMAP_LAYER,
  UNCLUSTERED_POINT_LAYER,
  MAP_STYLE_URL,
  INITIAL_VIEW,
  EVENT_TYPE_COLORS,
  EVENT_TYPE_LABELS,
  SEVERITY_COLORS,
  FALLBACK_COLOR,
} from "@/lib/mapStyles";
import type { EventsGeoJSON, EventProperties, Severity } from "@/lib/types";

interface MapViewProps {
  data: EventsGeoJSON | null;
  onBoundsChange: (bbox: [number, number, number, number]) => void;
}

// el builds a styled element whose text is assigned via textContent.
// Popup values originate from third-party feeds (GDACS descriptions carry
// HTML), so nothing feed-sourced may ever be interpolated into markup.
function el(tag: string, style: string, text?: string): HTMLElement {
  const node = document.createElement(tag);
  node.style.cssText = style;
  if (text !== undefined) node.textContent = text;
  return node;
}

function buildPopupContent(props: EventProperties): HTMLElement {
  const color = EVENT_TYPE_COLORS[props.event_type] ?? FALLBACK_COLOR;
  const label = EVENT_TYPE_LABELS[props.event_type] ?? props.event_type;
  const date = new Date(props.started_at).toLocaleString();

  const root = el("div", "max-width:280px;font-family:var(--font-sans)");

  const header = el(
    "div",
    "display:flex;align-items:center;gap:6px;margin-bottom:6px"
  );
  header.appendChild(
    el(
      "span",
      `width:10px;height:10px;border-radius:50%;background:${color};display:inline-block;flex-shrink:0`
    )
  );
  header.appendChild(
    el(
      "span",
      "font-size:11px;color:var(--color-foreground-muted);text-transform:uppercase;font-weight:600",
      label
    )
  );
  if (props.severity) {
    const badge = SEVERITY_COLORS[props.severity];
    if (badge) {
      header.appendChild(
        el(
          "span",
          `background:${badge};color:#161616;padding:2px 8px;font-size:11px;font-weight:600;text-transform:uppercase`,
          props.severity
        )
      );
    }
  }
  root.appendChild(header);

  root.appendChild(
    el(
      "div",
      "font-size:14px;font-weight:600;line-height:1.3;margin-bottom:4px;color:var(--color-foreground)",
      props.title
    )
  );

  if (props.magnitude != null) {
    const magnitude = el(
      "div",
      "font-size:13px;color:var(--color-foreground-muted);margin-top:4px",
      "Magnitude: "
    );
    const value = document.createElement("strong");
    value.textContent = String(props.magnitude);
    magnitude.appendChild(value);
    root.appendChild(magnitude);
  }

  root.appendChild(
    el("div", "font-size:12px;color:var(--color-foreground-muted);margin-top:4px", date)
  );

  if (props.description) {
    const truncated =
      props.description.length > 200
        ? `${props.description.slice(0, 200)}...`
        : props.description;
    root.appendChild(
      el(
        "div",
        "font-size:12px;color:var(--color-foreground-muted);margin-top:6px;line-height:1.4",
        truncated
      )
    );
  }

  const footer = el("div", "margin-top:8px");
  footer.appendChild(
    el("span", "font-size:11px;color:var(--color-foreground-faint)", `via ${props.source}`)
  );
  root.appendChild(footer);

  return root;
}

export default function MapView({ data, onBoundsChange }: MapViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const popupRef = useRef<maplibregl.Popup | null>(null);
  const styleReadyRef = useRef(false);
  const pendingDataRef = useRef<((map: maplibregl.Map) => void) | null>(null);
  const onBoundsChangeRef = useRef(onBoundsChange);
  useEffect(() => {
    onBoundsChangeRef.current = onBoundsChange;
  }, [onBoundsChange]);

  const setupLayers = useCallback((map: maplibregl.Map) => {
    if (map.getSource("events")) return;

    map.addSource("events", {
      type: "geojson",
      data: { type: "FeatureCollection", features: [] },
    });

    map.addLayer(HEATMAP_LAYER);
    map.addLayer(UNCLUSTERED_POINT_LAYER);
  }, []);

  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;

    // Reset per-map state: this effect re-runs on remount (React strict
    // mode runs it twice in dev), and a stale "ready" flag would let the
    // next map take data before its style exists.
    styleReadyRef.current = false;
    pendingDataRef.current = null;

    const map = new maplibregl.Map({
      container: containerRef.current,
      style: MAP_STYLE_URL,
      center: [INITIAL_VIEW.longitude, INITIAL_VIEW.latitude],
      zoom: INITIAL_VIEW.zoom,
    });

    map.addControl(new maplibregl.NavigationControl(), "bottom-right");
    map.addControl(
      new maplibregl.GeolocateControl({
        positionOptions: { enableHighAccuracy: true },
        trackUserLocation: false,
      }),
      "bottom-right"
    );

    const emitBounds = () => {
      const bounds = map.getBounds();
      onBoundsChangeRef.current([
        bounds.getWest(),
        bounds.getSouth(),
        bounds.getEast(),
        bounds.getNorth(),
      ]);
    };

    map.on("load", () => {
      styleReadyRef.current = true;
      setupLayers(map);
      emitBounds();
      // Apply whatever arrived while the style was still loading.
      pendingDataRef.current?.(map);
      pendingDataRef.current = null;
    });

    map.on("moveend", emitBounds);

    map.on("click", "unclustered-point", (e) => {
      const feature = e.features?.[0];
      if (!feature || feature.geometry.type !== "Point") return;

      const coords = feature.geometry.coordinates.slice() as [number, number];
      const props = feature.properties as Record<string, unknown>;

      const parsed: EventProperties = {
        id: String(props.id ?? ""),
        title: String(props.title ?? ""),
        event_type: String(props.event_type ?? "") as EventProperties["event_type"],
        source: String(props.source ?? ""),
        severity: props.severity ? (String(props.severity) as Severity) : undefined,
        magnitude: props.magnitude != null ? Number(props.magnitude) : undefined,
        started_at: String(props.started_at ?? ""),
        updated_at: String(props.updated_at ?? ""),
        url: props.url ? String(props.url) : undefined,
        description: props.description ? String(props.description) : undefined,
        metadata: props.metadata
          ? (typeof props.metadata === "string"
              ? JSON.parse(props.metadata)
              : props.metadata) as Record<string, unknown>
          : undefined,
      };

      popupRef.current?.remove();
      popupRef.current = new maplibregl.Popup({
        closeOnClick: true,
        maxWidth: "320px",
        offset: 12,
      })
        .setLngLat(coords)
        .setDOMContent(buildPopupContent(parsed))
        .addTo(map);
    });

    map.on("mouseenter", "unclustered-point", () => {
      map.getCanvas().style.cursor = "pointer";
    });
    map.on("mouseleave", "unclustered-point", () => {
      map.getCanvas().style.cursor = "";
    });

    mapRef.current = map;

    return () => {
      map.remove();
      mapRef.current = null;
      styleReadyRef.current = false;
      pendingDataRef.current = null;
    };
  }, [setupLayers]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !data) return;

    const applyData = (m: maplibregl.Map) => {
      if (!m.getSource("events")) {
        setupLayers(m);
      }
      const source = m.getSource("events") as maplibregl.GeoJSONSource;
      source.setData(data);
    };

    // Gate on our own load flag as well as isStyleLoaded(): the latter
    // reports false while tiles are still resolving even after "load" has
    // fired, and a once("load") registered at that point never runs — the
    // map then sits empty despite the data being there. If applying still
    // throws because the style isn't ready, fall back to the pending slot,
    // which the load handler flushes.
    if (styleReadyRef.current || map.isStyleLoaded()) {
      try {
        applyData(map);
      } catch {
        pendingDataRef.current = applyData;
      }
    } else {
      pendingDataRef.current = applyData;
    }
  }, [data, setupLayers]);

  return <div ref={containerRef} className="h-full w-full" />;
}
