import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { eventStyle, mapGround } from "./tokens";

describe("tokens", () => {
  it("gives every type its own color and shape from brand/tokens.json", () => {
    expect(eventStyle("flood", "light")).toEqual({ color: "#1b689c", shape: "triangle-down", label: "Flood" });
    expect(eventStyle("flood", "dark").color).toBe("#6db3e4");
  });

  it("falls back to other for a type it does not know", () => {
    expect(eventStyle("meteor", "light")).toEqual(eventStyle("other", "light"));
  });

  it("gives the map its grounds per theme", () => {
    expect(mapGround("light").ocean).toBe("#d9e7ea");
    expect(mapGround("dark").land).toBe("#1d2b2b");
  });
});

describe("_headers", () => {
  const headers = readFileSync(new URL("../../public/_headers", import.meta.url), "utf8");
  it("sends a Content-Security-Policy that lets the map run its workers", () => {
    const csp = headers.split("\n").find((l) => l.trim().startsWith("Content-Security-Policy:")) ?? "";
    for (const d of ["default-src 'self'", "worker-src 'self' blob:", "frame-ancestors 'none'", "object-src 'none'", "https://map.sentryatlas.com"]) {
      expect(csp).toContain(d);
    }
  });
  it("lets search engines index the site since the cutover (#40)", () => {
    expect(headers).not.toContain("noindex");
  });
});
