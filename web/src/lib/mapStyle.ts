import type { ExpressionSpecification, StyleSpecification } from "maplibre-gl";
import tokens from "../../../brand/tokens.json";
import type { Theme } from "./tokens";

/** OpenFreeMap's vector tiles, which have no service guarantee: a fork or a fallback sets its own. */
export const TILES_URL = process.env.NEXT_PUBLIC_TILES_URL ?? "https://tiles.openfreemap.org/planet";
export const GLYPHS_URL = process.env.NEXT_PUBLIC_GLYPHS_URL ?? "https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf";

const SOURCES_CREDIT = "Events: USGS, NASA EONET, NWS, GDACS";
const EMPTY = { type: "FeatureCollection" as const, features: [] };

/** Lines every 30 degrees, drawn faint like a survey sheet's grid. */
function graticule() {
  const features = [];
  for (let lon = -180; lon < 180; lon += 30) {
    features.push({ type: "Feature" as const, properties: {}, geometry: { type: "LineString" as const, coordinates: Array.from({ length: 37 }, (_, i) => [lon, -90 + i * 5]) } });
  }
  for (let lat = -60; lat <= 60; lat += 30) {
    features.push({ type: "Feature" as const, properties: {}, geometry: { type: "LineString" as const, coordinates: Array.from({ length: 73 }, (_, i) => [-180 + i * 5, lat]) } });
  }
  return { type: "FeatureCollection" as const, features };
}

/** Each feature's type color for a theme, as a style expression. */
export function typeColor(theme: Theme): ExpressionSpecification {
  const pairs = Object.entries(tokens.event).flatMap(([type, e]) => [type, e[theme]]);
  return ["match", ["get", "type"], ...pairs, tokens.event.other[theme]] as unknown as ExpressionSpecification;
}

/** The image name a mark layer asks for; MapView draws it when the map first needs it. */
export const markImageName = (theme: Theme): ExpressionSpecification => ["concat", `${theme}:`, ["get", "type"], ":", ["get", "fill"]];

/** The map's whole style for a theme: the basemap from the tokens, and the two event layers. */
export function mapStyle(theme: Theme): StyleSpecification {
  const c = tokens.color[theme];
  const quake = tokens.event.earthquake[theme];
  const q = tokens.mark["quake-radius"];
  // A ring's radius from its magnitude at world zoom (brand/BRAND.md), growing as the map zooms in.
  const ring = (z: number): ExpressionSpecification => ["*", z, ["max", q.min, ["*", ["-", ["coalesce", ["get", "mag"], 2.5], q["from-magnitude"]], q["per-magnitude"]]]];
  return {
    version: 8,
    projection: { type: "globe" },
    glyphs: GLYPHS_URL,
    sources: {
      openmaptiles: { type: "vector", url: TILES_URL },
      graticule: { type: "geojson", data: graticule() },
      quakes: {
        type: "geojson",
        data: EMPTY,
        cluster: true,
        clusterMaxZoom: 4,
        clusterRadius: 36,
        attribution: SOURCES_CREDIT,
      },
      marks: { type: "geojson", data: EMPTY },
      selected: { type: "geojson", data: EMPTY },
      areas: { type: "geojson", data: EMPTY },
      tracks: { type: "geojson", data: EMPTY },
    },
    layers: [
      { id: "land", type: "background", paint: { "background-color": c.land } },
      {
        id: "ocean",
        type: "fill",
        source: "openmaptiles",
        "source-layer": "water",
        filter: ["!=", ["get", "brunnel"], "tunnel"],
        paint: { "fill-color": c.ocean },
      },
      { id: "grid", type: "line", source: "graticule", paint: { "line-color": c.grid, "line-width": 0.6 } },
      {
        id: "coast",
        type: "line",
        source: "openmaptiles",
        "source-layer": "water",
        filter: ["!=", ["get", "brunnel"], "tunnel"],
        paint: { "line-color": c.coast, "line-width": ["interpolate", ["linear"], ["zoom"], 0, 0.4, 6, 1] },
      },
      {
        id: "borders",
        type: "line",
        source: "openmaptiles",
        "source-layer": "boundary",
        filter: ["all", ["==", ["get", "admin_level"], 2], ["!=", ["get", "maritime"], 1]],
        paint: { "line-color": c.coast, "line-width": ["interpolate", ["linear"], ["zoom"], 1, 0.3, 6, 0.9], "line-dasharray": [3, 2] },
      },
      // Areas and tracks from zoom 3, under the names so the names stay readable. The fill is
      // faint enough that overlapping areas still show each other's edges.
      { id: "area-fill", type: "fill", source: "areas", minzoom: 3, paint: { "fill-color": typeColor(theme), "fill-opacity": 0.12 } },
      { id: "area-line", type: "line", source: "areas", minzoom: 3, paint: { "line-color": typeColor(theme), "line-width": 1 } },
      { id: "tracks", type: "line", source: "tracks", minzoom: 2, layout: { "line-cap": "round" }, paint: { "line-color": typeColor(theme), "line-width": 2, "line-dasharray": [2, 1.5] } },
      // The selected event's area or track, at any zoom.
      { id: "selected-area", type: "line", source: "areas", filter: ["==", ["get", "id"], ""], paint: { "line-color": c.ink, "line-width": 2.5 } },
      { id: "selected-track", type: "line", source: "tracks", filter: ["==", ["get", "id"], ""], paint: { "line-color": c.ink, "line-width": 3 } },
      {
        id: "places",
        type: "symbol",
        source: "openmaptiles",
        "source-layer": "place",
        filter: ["any", ["==", ["get", "class"], "country"], ["all", ["==", ["get", "class"], "city"], [">=", ["zoom"], 4]]],
        layout: {
          "text-field": ["coalesce", ["get", "name:en"], ["get", "name"]],
          "text-font": ["Noto Sans Regular"],
          "text-size": ["interpolate", ["linear"], ["zoom"], 1, 10, 6, 13],
          "text-transform": ["case", ["==", ["get", "class"], "country"], "uppercase", "none"],
          "text-letter-spacing": ["case", ["==", ["get", "class"], "country"], 0.12, 0],
        },
        paint: { "text-color": c.muted, "text-halo-color": c.sheet, "text-halo-width": 1.2 },
      },
      // Clusters hold ungraded quakes only, so a graded event of any type never hides inside one.
      {
        id: "quake-clusters",
        type: "circle",
        source: "quakes",
        filter: ["has", "point_count"],
        paint: {
          "circle-radius": ["step", ["get", "point_count"], 9, 25, 12, 200, 16],
          "circle-color": c.sheet,
          "circle-opacity": 0.85,
          "circle-stroke-color": quake,
          "circle-stroke-width": 1.4,
        },
      },
      {
        id: "quake-cluster-count",
        type: "symbol",
        source: "quakes",
        filter: ["has", "point_count"],
        layout: { "text-field": ["get", "point_count_abbreviated"], "text-font": ["Noto Sans Bold"], "text-size": 11, "text-allow-overlap": true },
        paint: { "text-color": c.ink },
      },
      {
        id: "quakes",
        type: "circle",
        source: "quakes",
        filter: ["!", ["has", "point_count"]],
        paint: {
          "circle-radius": ["interpolate", ["exponential", 1.6], ["zoom"], 0, ring(1), 8, ring(4)],
          "circle-color": "rgba(0,0,0,0)",
          "circle-stroke-color": quake,
          "circle-stroke-width": tokens.mark["quake-stroke"],
          "circle-stroke-opacity": tokens.mark["quake-opacity"],
        },
      },
      {
        id: "marks",
        type: "symbol",
        source: "marks",
        layout: {
          "icon-image": markImageName(theme),
          "icon-size": ["interpolate", ["linear"], ["zoom"], 0, 0.8, 6, 1, 10, 1.3],
          "icon-allow-overlap": true,
          "icon-ignore-placement": true,
          // The most severe draws on top.
          "symbol-sort-key": ["get", "rank"],
        },
      },
      // The selected event: a ring in ink around its mark.
      {
        id: "selected",
        type: "circle",
        source: "selected",
        paint: { "circle-radius": 13, "circle-color": "rgba(0,0,0,0)", "circle-stroke-color": c.ink, "circle-stroke-width": 2 },
      },
    ],
  };
}
