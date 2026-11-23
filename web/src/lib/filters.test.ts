import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { Event, Snapshot } from "./api.gen";
import { filterEvents, freshness, inBBox, position, PRESETS, type EventType } from "./filters";

const example = JSON.parse(readFileSync(new URL("../../../docs/api/v2/snapshot.example.json", import.meta.url), "utf8")) as Snapshot;
const ids = (events: Event[]) => events.map((e) => e.id.split("-")[0] + ":" + e.type);

describe("inBBox", () => {
  it("handles a plain box", () => {
    expect(inBBox([10, 10], [0, 0, 20, 20])).toBe(true);
    expect(inBBox([25, 10], [0, 0, 20, 20])).toBe(false);
    expect(inBBox([10, 25], [0, 0, 20, 20])).toBe(false);
  });
  it("handles a box across the date line, written either way", () => {
    for (const box of [[170, -30, 190, 0], [170, -30, -170, 0]] as const) {
      expect(inBBox([175, -10], [...box])).toBe(true);
      expect(inBBox([-175, -10], [...box])).toBe(true);
      expect(inBBox([0, -10], [...box])).toBe(false);
    }
  });
  it("treats 360 degrees or more as the whole world", () => {
    expect(inBBox([123, 5], [-200, -90, 200, 90])).toBe(true);
  });
});

describe("filterEvents", () => {
  const all = example.events;

  it("keeps everything with no filters", () => {
    expect(filterEvents(all, {})).toHaveLength(all.length);
  });

  it("keeps only the chosen types", () => {
    expect(ids(filterEvents(all, { types: new Set<EventType>(["weather"]) }))).toEqual(["noaa:weather", "noaa:weather"]);
  });

  it("keeps events that began at or after the time", () => {
    const since = Date.parse("2026-11-22T20:12:00Z");
    expect(ids(filterEvents(all, { since }))).toEqual(["noaa:weather", "noaa:weather"]);
  });

  it("keeps events placed in the view and leaves out those with no place", () => {
    // The Bay of Bengal: the EONET storm and the GDACS cyclone.
    expect(ids(filterEvents(all, { bbox: [75, 10, 90, 20] }))).toEqual(["eonet:storm", "gdacs:cyclone"]);
    expect(filterEvents(all, { bbox: [-180, -90, 180, 90] })).toHaveLength(all.filter((e) => position(e)).length);
  });

  it("draws one member per group: the primary", () => {
    const quake = all[0];
    const grouped: Event[] = [
      { ...quake, id: "usgs-1", group_id: "g", primary: true },
      { ...quake, id: "gdacs-EQ-1", source: "gdacs", group_id: "g" },
      { ...quake, id: "usgs-2" },
    ];
    expect(filterEvents(grouped, {}).map((e) => e.id)).toEqual(["usgs-1", "usgs-2"]);
  });

  it("combines filters", () => {
    const f = { types: new Set<EventType>(["cyclone", "storm"]), since: Date.parse("2026-11-10T00:00:00Z"), bbox: [75, 10, 90, 20] as [number, number, number, number] };
    expect(ids(filterEvents(all, f))).toEqual(["eonet:storm"]);
  });
});

describe("PRESETS", () => {
  it("asks for the 30-day snapshot only for 30 days", () => {
    expect(PRESETS["24h"].window).toBe("7d");
    expect(PRESETS["7d"].window).toBe("7d");
    expect(PRESETS["30d"].window).toBe("30d");
  });
});

describe("freshness", () => {
  it("is not partial when every source answered", () => {
    const f = freshness(example, Date.parse("2026-11-22T21:00:00Z"));
    expect(f.partial).toBe(false);
    expect(f.sources.find((s) => s.source === "gdacs")?.silentMinutes).toBe(4);
  });

  it("is partial when a source is stale or down, and says how long it has been silent", () => {
    const s: Snapshot = {
      ...example,
      sources: [
        { source: "noaa", ok: false, stale: true, fetched_at: "2026-11-22T20:40:00Z", count: 2, error: "timeout" },
        { source: "usgs", ok: false, stale: false, count: 0 },
      ],
    };
    const f = freshness(s, Date.parse("2026-11-22T21:00:30Z"));
    expect(f.partial).toBe(true);
    expect(f.sources[0]).toMatchObject({ stale: true, silentMinutes: 20 });
    expect(f.sources[1].silentMinutes).toBeUndefined();
  });
});
