import { describe, expect, it } from "vitest";
import { fetchDetail } from "./detail";

const answer = (status: number, body: unknown = {}) => (async () => new Response(JSON.stringify(body), { status })) as typeof fetch;

describe("fetchDetail", () => {
  it("encodes the id, so an NWS id with colons reaches the API as one segment", async () => {
    let asked = "";
    const f = (async (url: string) => {
      asked = url;
      return new Response(JSON.stringify({ event: { id: "x" }, source: { source: "noaa" } }));
    }) as unknown as typeof fetch;
    const d = await fetchDetail("noaa-urn:oid:2.49.0.1", f);
    expect(asked).toMatch(/\/api\/v2\/events\/noaa-urn%3Aoid%3A2\.49\.0\.1$/);
    expect(d.status).toBe("ok");
  });

  it("says an event has ended when the API no longer serves it", async () => {
    expect(await fetchDetail("usgs-gone", answer(404))).toEqual({ status: "ended" });
  });

  it("says error for anything else, and for a network failure", async () => {
    expect(await fetchDetail("usgs-x", answer(503))).toEqual({ status: "error" });
    const down = (async () => {
      throw new TypeError("offline");
    }) as typeof fetch;
    expect(await fetchDetail("usgs-x", down)).toEqual({ status: "error" });
  });
});
