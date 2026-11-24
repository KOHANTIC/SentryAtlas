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
  // Each rule's path, and the header lines indented under it.
  const rules = new Map<string, string[]>();
  let path = "";
  for (const line of headers.split("\n")) {
    if (!line.trim() || line.startsWith("#")) continue;
    if (!line.startsWith(" ")) rules.set((path = line.trim()), []);
    else rules.get(path)?.push(line.trim());
  }
  const csp = (rule: string) => rules.get(rule)?.find((l) => l.startsWith("Content-Security-Policy:")) ?? "";

  it("sends a Content-Security-Policy that lets the map run its workers", () => {
    for (const d of ["default-src 'self'", "worker-src 'self' blob:", "frame-ancestors 'none'", "object-src 'none'", "https://map.sentryatlas.com"]) {
      expect(csp("/*")).toContain(d);
    }
  });
  it("lets the map fetch radar and satellite tiles (#48)", () => {
    const directive = (name: string) => csp("/*").split(";").map((d) => d.trim().split(" ")).find((d) => d[0] === name || d[1] === name) ?? [];
    for (const name of ["img-src", "connect-src"]) {
      expect(directive(name)).toEqual(expect.arrayContaining(["https://nowcoast.noaa.gov", "https://gibs.earthdata.nasa.gov"]));
    }
  });
  it("lets other sites frame the embed (#50), with the same policy otherwise", () => {
    const embed = rules.get("/embed/") ?? [];
    // Rules that match together join their values, so the embed must drop the policy that refuses frames.
    expect(embed[0]).toBe("! Content-Security-Policy");
    expect(csp("/embed/")).toBe(csp("/*").replace("frame-ancestors 'none'", "frame-ancestors *"));
    expect(csp("/embed/")).toContain("frame-ancestors *;");
  });
  it("lets search engines index the site since the cutover (#40)", () => {
    expect(headers).not.toContain("noindex");
  });
});
