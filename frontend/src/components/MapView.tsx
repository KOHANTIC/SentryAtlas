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
} from "@/lib/mapStyles";
import type { EventsGeoJSON, EventProperties, Severity } from "@/lib/types";

interface MapViewProps {
  data: EventsGeoJSON | null;
  onSelectEvent: (properties: EventProperties | null) => void;
}

const SEVERITY_BADGES: Record<Severity, { bg: string; text: string }> = {
  extreme: { bg: "#dc2626", text: "#fff" },
  severe: { bg: "#ea580c", text: "#fff" },
  moderate: { bg: "#ca8a04", text: "#fff" },
  minor: { bg: "#16a34a", text: "#fff" },
};

function buildPopupHTML(props: EventProperties): string {
  const color = EVENT_TYPE_COLORS[props.event_type] ?? "#888";
  const label = EVENT_TYPE_LABELS[props.event_type] ?? props.event_type;
  const date = new Date(props.started_at).toLocaleString();

  let severityBadge = "";
  if (props.severity) {
    const badge = SEVERITY_BADGES[props.severity];
    if (badge) {
      severityBadge = `<span style="background:${badge.bg};color:${badge.text};padding:2px 8px;border-radius:9999px;font-size:11px;font-weight:600;text-transform:uppercase">${props.severity}</span>`;
    }
  }

  let magnitudeText = "";
  if (props.magnitude != null) {
    magnitudeText = `<div style="font-size:13px;color:#555;margin-top:4px">Magnitude: <strong>${props.magnitude}</strong></div>`;
  }

  return `
    <div style="max-width:280px;font-family:system-ui,sans-serif">
      <div style="display:flex;align-items:center;gap:6px;margin-bottom:6px">
        <span style="width:10px;height:10px;border-radius:50%;background:${color};display:inline-block;flex-shrink:0"></span>
        <span style="font-size:11px;color:#666;text-transform:uppercase;font-weight:600">${label}</span>
        ${severityBadge}
      </div>
      <div style="font-size:14px;font-weight:600;line-height:1.3;margin-bottom:4px;color:#1a1a2e">${props.title}</div>
      ${magnitudeText}
      <div style="font-size:12px;color:#777;margin-top:4px">${date}</div>
      ${props.description ? `<div style="font-size:12px;color:#555;margin-top:6px;line-height:1.4">${props.description.slice(0, 200)}${props.description.length > 200 ? "..." : ""}</div>` : ""}
      <div style="margin-top:8px">
        <span style="font-size:11px;color:#999">via ${props.source}</span>
      </div>
    </div>
  `;
}

export default function MapView({ data, onSelectEvent }: MapViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const popupRef = useRef<maplibregl.Popup | null>(null);

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

    map.on("load", () => {
      setupLayers(map);
    });

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

      onSelectEvent(parsed);

      popupRef.current?.remove();
      popupRef.current = new maplibregl.Popup({
        closeOnClick: true,
        maxWidth: "320px",
        offset: 12,
      })
        .setLngLat(coords)
        .setHTML(buildPopupHTML(parsed))
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
    };
  }, [setupLayers, onSelectEvent]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !data) return;

    const applyData = () => {
      if (!map.getSource("events")) {
        setupLayers(map);
      }
      const source = map.getSource("events") as maplibregl.GeoJSONSource;
      source.setData(data);
    };

    if (map.isStyleLoaded()) {
      applyData();
    } else {
      map.once("load", applyData);
    }
  }, [data, setupLayers]);

  return <div ref={containerRef} className="h-full w-full" />;
}
