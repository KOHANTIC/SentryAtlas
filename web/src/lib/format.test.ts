import { describe, expect, it } from "vitest";
import { formatMagnitude, formatSeverity, formatTime, paragraphs } from "./format";

describe("formatTime", () => {
  it("gives the viewer's zone with UTC beside it", () => {
    expect(formatTime("2026-11-22T22:27:00Z", "America/Los_Angeles")).toBe("22 Nov 2026, 14:27 PST (22:27 UTC)");
  });
  it("says it once for a viewer in UTC", () => {
    expect(formatTime("2026-11-22T22:27:00Z", "UTC")).toBe("22 Nov 2026, 22:27 UTC");
  });
});

describe("formatMagnitude", () => {
  it("writes a quake's magnitude with its type, and other values with their unit", () => {
    expect(formatMagnitude({ type: "earthquake", magnitude: { value: 5.23, unit: "mww" } })).toBe("M 5.2 (mww)");
    expect(formatMagnitude({ type: "cyclone", magnitude: { value: 120, unit: "kts" } })).toBe("120 kts");
    expect(formatMagnitude({ type: "wildfire", magnitude: { value: 12500, unit: "acres" } })).toBe("12,500 acres");
    expect(formatMagnitude({ type: "flood" })).toBeUndefined();
  });
});

describe("formatSeverity", () => {
  it("puts the source's own word beside the level", () => {
    expect(formatSeverity({ source: "noaa", severity: { level: "severe", source_level: "Severe" } })).toBe("Severe (NWS: Severe)");
    expect(formatSeverity({ source: "gdacs", severity: { level: "moderate", source_level: "Orange" } })).toBe("Moderate (GDACS: Orange)");
    expect(formatSeverity({ source: "eonet" })).toBe("Not graded");
  });
});

describe("paragraphs", () => {
  it("keeps every word, joins NWS's wrapped lines, and parts paragraphs at blank lines", () => {
    const nws = "If driving, slow down, use your headlights, and leave plenty of\ndistance ahead of you.\n\n* WHAT...Visibility one-quarter mile\nor less.\n";
    expect(paragraphs(nws)).toEqual([
      "If driving, slow down, use your headlights, and leave plenty of distance ahead of you.",
      "* WHAT...Visibility one-quarter mile or less.",
    ]);
    const words = (s: string) => s.split(/\s+/).filter(Boolean);
    expect(words(paragraphs(nws).join(" "))).toEqual(words(nws));
  });
});
