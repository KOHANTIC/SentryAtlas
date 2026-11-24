import { describe, expect, it } from "vitest";
import { parse, serialize, type ViewState } from "./urlState";

describe("URL state", () => {
  it("round-trips a full view", () => {
    const v: ViewState = { center: [-122.4194, 37.7749], zoom: 6.25, bearing: 30, types: ["flood", "wildfire"], time: "30d", event: "usgs-ak0261d6m9t2" };
    const s = serialize(v);
    expect(s).toBe("?at=37.7749,-122.4194,6.25,30&types=flood,wildfire&time=30d&event=usgs-ak0261d6m9t2");
    expect(parse(s)).toEqual(v);
  });

  it("leaves defaults out, so the plain map is a plain link", () => {
    expect(serialize({ time: "7d" })).toBe("");
    expect(parse("")).toEqual({ time: "7d" });
    expect(serialize({ center: [10, 20], zoom: 2, bearing: 0, time: "7d" })).toBe("?at=20,10,2");
  });

  it("rounds so a moved map gives a short link", () => {
    expect(serialize({ center: [10.123456789, 20.987654321], zoom: 2.34567, time: "7d" })).toBe("?at=20.9877,10.1235,2.35");
  });

  it("drops what it cannot read instead of failing", () => {
    expect(parse("?at=abc,1,2&types=meteor,flood,flood&time=1y&event=nope")).toEqual({ time: "7d", types: ["flood"] });
    expect(parse("?at=95,200,30")).toEqual({ time: "7d", center: [-160, 85], zoom: 22 });
  });

  it("narrows the window by hours inside its preset, and drops hours outside it", () => {
    expect(parse(serialize({ time: "30d", hours: 36 }))).toEqual({ time: "30d", hours: 36 });
    expect(serialize({ time: "7d", hours: 36 })).toBe("?hours=36");
    // The whole preset, or more, is no narrowing; zero or less means nothing.
    expect(parse("?time=24h&hours=24")).toEqual({ time: "24h" });
    expect(parse("?hours=0")).toEqual({ time: "7d" });
    expect(parse("?hours=abc")).toEqual({ time: "7d" });
  });

  it("carries the layers that are on, in one order, and drops names it does not know", () => {
    expect(serialize({ time: "30d", layers: ["satellite", "radar"], event: "usgs-x" })).toBe("?time=30d&layers=radar,satellite&event=usgs-x");
    expect(parse("?layers=satellite,clouds,satellite")).toEqual({ time: "7d", layers: ["satellite"] });
    expect(parse("?layers=clouds")).toEqual({ time: "7d" });
    expect(serialize({ time: "7d", layers: [] })).toBe("");
  });

  it("keeps NWS ids, which hold colons, through a round trip", () => {
    const event = "noaa-urn:oid:2.49.0.1.840.0.d5372f24080570d0330457c696ac9389960c1ab5.001.1";
    expect(parse(serialize({ time: "7d", event })).event).toBe(event);
  });
});
