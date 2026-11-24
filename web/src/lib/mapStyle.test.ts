import { describe, expect, it } from "vitest";
import { mapStyle } from "./mapStyle";

const ids = (s: ReturnType<typeof mapStyle>) => s.layers.map((l) => l.id);

describe("mapStyle imagery", () => {
  it("has no imagery source while the layers are off, so neither host is asked", () => {
    const s = mapStyle("light");
    expect(Object.keys(s.sources)).not.toContain("radar");
    expect(Object.keys(s.sources)).not.toContain("satellite");
    expect(JSON.stringify(s)).not.toMatch(/nowcoast|gibs/);
  });

  it("puts the satellite photo under the grid and coasts, and radar under the warning areas and marks", () => {
    const s = mapStyle("dark", { radar: 1, satellite: "2026-11-21" });
    const order = ids(s);
    expect(order.indexOf("ocean")).toBeLessThan(order.indexOf("satellite"));
    expect(order.indexOf("satellite")).toBeLessThan(order.indexOf("grid"));
    expect(order.indexOf("borders")).toBeLessThan(order.indexOf("radar"));
    expect(order.indexOf("radar")).toBeLessThan(order.indexOf("area-fill"));
    expect(order.indexOf("radar")).toBeLessThan(order.indexOf("marks"));
  });

  it("turns each layer on alone", () => {
    expect(ids(mapStyle("light", { radar: 1 }))).toContain("radar");
    expect(ids(mapStyle("light", { radar: 1 }))).not.toContain("satellite");
    expect(ids(mapStyle("light", { satellite: "2026-11-21" }))).not.toContain("radar");
  });
});
