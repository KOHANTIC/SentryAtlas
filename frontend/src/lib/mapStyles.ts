import type { EventType } from "./types";
import type {
  CircleLayerSpecification,
  SymbolLayerSpecification,
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

export const CLUSTER_LAYER: CircleLayerSpecification = {
  id: "clusters",
  type: "circle",
  source: "events",
  filter: ["has", "point_count"],
  paint: {
    "circle-color": [
      "step",
      ["get", "point_count"],
      "#51bbd6",
      10,
      "#f1f075",
      50,
      "#f28cb1",
    ],
    "circle-radius": ["step", ["get", "point_count"], 18, 10, 24, 50, 32],
    "circle-stroke-width": 2,
    "circle-stroke-color": "#ffffff",
  },
};

export const CLUSTER_COUNT_LAYER: SymbolLayerSpecification = {
  id: "cluster-count",
  type: "symbol",
  source: "events",
  filter: ["has", "point_count"],
  layout: {
    "text-field": "{point_count_abbreviated}",
    "text-size": 13,
    "text-font": ["Open Sans Bold"],
  },
  paint: {
    "text-color": "#1a1a2e",
  },
};

export const UNCLUSTERED_POINT_LAYER: CircleLayerSpecification = {
  id: "unclustered-point",
  type: "circle",
  source: "events",
  filter: ["!", ["has", "point_count"]],
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
    "circle-opacity": 0.9,
  },
};

export const MAP_STYLE_URL = "https://tiles.openfreemap.org/styles/liberty";

export const INITIAL_VIEW = {
  longitude: 0,
  latitude: 20,
  zoom: 2,
};
