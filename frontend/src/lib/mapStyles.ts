import type { EventType } from "./types";
import type {
  CircleLayerSpecification,
  HeatmapLayerSpecification,
  DataDrivenPropertyValueSpecification,
} from "maplibre-gl";

export const EVENT_TYPE_COLORS: Record<EventType, string> = {
  earthquake: "#e74c3c",
  wildfire: "#e67e22",
  volcano: "#c0392b",
  storm: "#3498db",
  flood: "#2980b9",
  cyclone: "#8e44ad",
  tornado: "#9b59b6",
  hurricane: "#2c3e50",
  winter_storm: "#95a5a6",
  tsunami: "#1abc9c",
  drought: "#f39c12",
  iceberg: "#7fb3d3",
  landslide: "#d35400",
};

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
};

const colorMatchExpression = [
  "match",
  ["get", "event_type"],
  "earthquake", "#e74c3c",
  "wildfire", "#e67e22",
  "volcano", "#c0392b",
  "storm", "#3498db",
  "flood", "#2980b9",
  "cyclone", "#8e44ad",
  "tornado", "#9b59b6",
  "hurricane", "#2c3e50",
  "winter_storm", "#95a5a6",
  "tsunami", "#1abc9c",
  "drought", "#f39c12",
  "iceberg", "#7fb3d3",
  "landslide", "#d35400",
  "#888888",
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
      0.1, "#16a34a",
      0.3, "#a3e635",
      0.5, "#facc15",
      0.7, "#f97316",
      1, "#dc2626",
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
    "circle-stroke-width": 2,
    "circle-stroke-color": "#ffffff",
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

export const MAP_STYLE_URL = "https://tiles.openfreemap.org/styles/liberty";

export const INITIAL_VIEW = {
  longitude: 0,
  latitude: 20,
  zoom: 2,
};
