import type { EventType, Severity } from "./types";
import type {
  CircleLayerSpecification,
  HeatmapLayerSpecification,
  DataDrivenPropertyValueSpecification,
} from "maplibre-gl";

/*
 * Event-type palette — the single source of truth. The MapLibre match
 * expression, the legend, and the filter panel all derive from this record;
 * nothing else may restate a type color.
 *
 * Designed for the dark surface (#161616) with semantic hue families
 * (fire/earth warm, water blue, wind violet, ice cyan/teal, dry gold) and
 * optimized so the worst of all 78 chromatic pairs keeps OKLab ΔE ≥ 8.7
 * under normal vision, with every color ≥ 3:1 contrast on the surface.
 * Full pairwise CVD distinctness is unreachable at 13 chromatic categories
 * (collapses stay within a hue family); identity is therefore never
 * color-alone — the legend, filter panel, and popups all carry the type
 * name, and the filter can isolate any single type.
 * "weather" and "other" are deliberately low-chroma fallback slots.
 */
export const EVENT_TYPE_COLORS: Record<EventType, string> = {
  earthquake: "#d17714",
  wildfire: "#fd4812",
  volcano: "#d0374c",
  landslide: "#8f5d14",
  drought: "#a78e2a",
  storm: "#1f9dd4",
  flood: "#1962f0",
  tsunami: "#0092a4",
  winter_storm: "#0683df",
  iceberg: "#1baa86",
  hurricane: "#6f5bbd",
  cyclone: "#9f3bbb",
  tornado: "#d94e9a",
  weather: "#93a1b0",
  other: "#8c8c8c",
};

export const FALLBACK_COLOR = "#8c8c8c";

export const EVENT_TYPE_LABELS: Record<EventType, string> = {
  earthquake: "Earthquake",
  wildfire: "Wildfire",
  volcano: "Volcano",
  storm: "Storm",
  flood: "Flood",
  cyclone: "Cyclone",
  tornado: "Tornado",
  hurricane: "Hurricane",
  winter_storm: "Winter Storm",
  tsunami: "Tsunami",
  drought: "Drought",
  iceberg: "Iceberg",
  landslide: "Landslide",
  weather: "Weather",
  other: "Other",
};

/*
 * Severity badge colors (status scale, not categorical): brand feedback
 * tokens — success green, accent amber pair, error red. Badges always
 * carry their text label, and #161616 text clears 4.5:1 on all four.
 */
export const SEVERITY_COLORS: Record<Severity, string> = {
  minor: "#22c55e",
  moderate: "#f6ad65",
  severe: "#e07a1e",
  extreme: "#ef4444",
};

/*
 * Heat ramp: the KOHANTIC accent ramp itself (900 → 100), so density reads
 * as an amber glow rising out of the dark surface. Single hue family,
 * monotone lightness — CVD-stable by construction.
 */
export const SEVERITY_RAMP: ReadonlyArray<{ stop: number; color: string }> = [
  { stop: 0.1, color: "#5c2d08" },
  { stop: 0.3, color: "#b85f14" },
  { stop: 0.5, color: "#e07a1e" },
  { stop: 0.75, color: "#f39540" },
  { stop: 1, color: "#fdebd4" },
];

export const SEVERITY_RAMP_GRADIENT = `linear-gradient(to right, ${SEVERITY_RAMP.map(
  (s) => s.color
).join(", ")})`;

const colorMatchExpression = [
  "match",
  ["get", "event_type"],
  ...Object.entries(EVENT_TYPE_COLORS).flat(),
  FALLBACK_COLOR,
] as unknown as DataDrivenPropertyValueSpecification<string>;

export const HEATMAP_LAYER = {
  id: "events-heat",
  type: "heatmap",
  source: "events",
  paint: {
    "heatmap-weight": [
      "match",
      ["get", "severity"],
      "extreme", 1,
      "severe", 0.75,
      "moderate", 0.5,
      "minor", 0.25,
      0.3,
    ],
    "heatmap-intensity": [
      "interpolate", ["linear"], ["zoom"],
      0, 1,
      9, 3,
    ],
    "heatmap-color": [
      "interpolate", ["linear"], ["heatmap-density"],
      0, "rgba(0,0,0,0)",
      ...SEVERITY_RAMP.flatMap(({ stop, color }) => [stop, color]),
    ],
    "heatmap-radius": [
      "interpolate", ["linear"], ["zoom"],
      0, 15,
      9, 30,
    ],
    "heatmap-opacity": [
      "interpolate", ["linear"], ["zoom"],
      7, 0.8,
      9, 0,
    ],
  },
} as unknown as HeatmapLayerSpecification;

export const UNCLUSTERED_POINT_LAYER: CircleLayerSpecification = {
  id: "unclustered-point",
  type: "circle",
  source: "events",
  paint: {
    "circle-color": colorMatchExpression,
    "circle-radius": [
      "interpolate",
      ["linear"],
      ["coalesce", ["get", "magnitude"], 3],
      0,
      5,
      5,
      10,
      8,
      16,
    ],
    // Surface-colored ring: separates overlapping marks without adding a
    // bright halo on the dark basemap.
    "circle-stroke-width": 1.5,
    "circle-stroke-color": "#161616",
    "circle-opacity": [
      "interpolate", ["linear"], ["zoom"],
      6, 0,
      8, 0.9,
    ],
    "circle-stroke-opacity": [
      "interpolate", ["linear"], ["zoom"],
      6, 0,
      8, 1,
    ],
  },
};

export const MAP_STYLE_URL = "https://tiles.openfreemap.org/styles/dark";

export const INITIAL_VIEW = {
  longitude: 0,
  latitude: 20,
  zoom: 2,
};
