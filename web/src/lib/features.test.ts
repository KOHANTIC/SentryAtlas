import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import tokens from "../../../brand/tokens.json";
import type { Event, Snapshot } from "./api.gen";
import { toAreas, toFeatures, toTracks, zoneOnly } from "./features";
import { mapStyle, typeColor } from "./mapStyle";
import { svgPath, type Shape } from "./marks";

const example = JSON.parse(readFileSync(new URL("../../../docs/api/v2/snapshot.example.json", import.meta.url), "utf8")) as Snapshot;

describe("toFeatures", () => {
  it("puts ungraded quakes in the clustered source and every other placed event in the marks", () => {
    const { quakes, marks } = toFeatures(example.events);
    expect(quakes.features.map((f) => f.properties.id)).toEqual(["usgs-aka2026xaubpp"]);
    // The NWS alert with no place is left out.
    expect(marks.features.map((f) => f.properties.type)).toEqual(["storm", "weather", "cyclone"]);
  });

  it("gives each mark its severity's fill and rank, and quakes their magnitude", () => {
    const quake = example.events[0];
    const graded: Event = { ...quake, id: "usgs-x", severity: { level: "extreme", source_level: "red" } };
    const { quakes, marks } = toFeatures([quake, graded]);
    expect(quakes.features[0].properties).toMatchObject({ fill: "dotted", rank: 0, mag: quake.magnitude?.value });
    // A graded quake leaves the clusters, so it can never hide inside one.
    expect(marks.features[0].properties).toMatchObject({ id: "usgs-x", fill: "halo", rank: 4 });
  });
});

describe("svgPath", () => {
  it("draws every shape the tokens use, inside the mark's box", () => {
    const shapes = new Set(Object.values(tokens.event).map((e) => e.shape as Shape));
    for (const s of shapes) {
      const d = svgPath(s);
      expect(d).toMatch(/^M.*Z$/);
      for (const n of d.match(/-?\d+(\.\d+)?/g)!.map(Number)) expect(Math.abs(n)).toBeLessThanOrEqual(6);
    }
  });
});

describe("mapStyle", () => {
  it("is a globe with the theme's grounds, and asks for marks by theme", () => {
    for (const theme of ["light", "dark"] as const) {
      const s = mapStyle(theme);
      expect(s.projection).toEqual({ type: "globe" });
      const paint = (id: string) => (s.layers.find((l) => l.id === id) as { paint: Record<string, unknown> }).paint;
      expect(paint("land")["background-color"]).toBe(tokens.color[theme].land);
      expect(paint("ocean")["fill-color"]).toBe(tokens.color[theme].ocean);
      expect(JSON.stringify(s.layers.find((l) => l.id === "marks"))).toContain(`${theme}:`);
    }
  });

  it("credits the event sources, and draws events above the basemap", () => {
    const s = mapStyle("light");
    expect(JSON.stringify(s.sources.quakes)).toContain("USGS, NASA EONET, NWS, GDACS");
    const ids = s.layers.map((l) => l.id);
    expect(ids.indexOf("marks")).toBeGreaterThan(ids.indexOf("places"));
  });
});

describe("areas and tracks", () => {
  const [, storm, polygon, zoned] = example.events;
  const square = [[[[-87, 42], [-86, 42], [-86, 43], [-87, 43], [-87, 42]]]];

  it("draws an alert's own polygon, and a zone-only alert from its zones once they load", () => {
    expect(zoneOnly(zoned)).toBe(true);
    expect(zoneOnly(polygon)).toBe(false);
    expect(toAreas(example.events).features.map((f) => f.properties.id)).toEqual([polygon.id]);
    const zones = new Map([["LMZ844", square], ["LMZ845", square]]);
    const areas = toAreas(example.events, zones).features;
    expect(areas.map((f) => f.properties.id)).toEqual([polygon.id, zoned.id]);
    // LMZ846 is not in this table, so it is skipped, not an error.
    expect(areas[1].geometry.coordinates).toHaveLength(2);
  });

  it("draws a track as a line", () => {
    expect(toTracks(example.events).features.map((f) => f.properties.id)).toEqual([storm.id]);
  });

  it("colors areas and tracks by type, for every type", () => {
    const expr = JSON.stringify(typeColor("light"));
    for (const [type, e] of Object.entries(tokens.event)) expect(expr).toContain(`"${type}","${e.light}"`);
  });
});
