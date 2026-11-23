import { describe, expect, it } from "vitest";
import type { Event } from "./api.gen";
import { sortEvents } from "./sort";

const ev = (id: string, updated: string, level?: "extreme" | "severe" | "moderate" | "minor"): Event =>
  ({ id, time: { started: updated, updated }, severity: level && { level, source_level: level } }) as Event;

describe("sortEvents", () => {
  const list = [ev("a", "2026-11-20T00:00:00Z", "minor"), ev("b", "2026-11-22T00:00:00Z"), ev("c", "2026-11-21T00:00:00Z", "severe"), ev("d", "2026-11-19T00:00:00Z", "severe")];
  it("puts the newest first", () => {
    expect(sortEvents(list, "time").map((e) => e.id)).toEqual(["b", "c", "a", "d"]);
  });
  it("puts the most severe first, newest first within a level, ungraded last", () => {
    expect(sortEvents(list, "severity").map((e) => e.id)).toEqual(["c", "d", "a", "b"]);
  });
  it("leaves its input alone", () => {
    sortEvents(list, "severity");
    expect(list.map((e) => e.id)).toEqual(["a", "b", "c", "d"]);
  });
});
