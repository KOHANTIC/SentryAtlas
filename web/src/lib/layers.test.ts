import { describe, expect, it } from "vitest";
import { radarStamp, radarTiles, satelliteDay, satelliteTiles } from "./layers";

describe("radarTiles", () => {
  it("asks nowCOAST for all five US mosaics in the map's projection, with the stamp", () => {
    const url = new URL(radarTiles(5_901_234).replace("{bbox-epsg-3857}", "0,0,1,1"));
    expect(url.host).toBe("nowcoast.noaa.gov");
    expect(url.searchParams.get("layers")?.split(",")).toEqual([
      "base_reflectivity_mosaic",
      "alaska_base_reflectivity_mosaic",
      "hawaii_base_reflectivity_mosaic",
      "caribbean_base_reflectivity_mosaic",
      "guam_base_reflectivity_mosaic",
    ]);
    expect(url.searchParams.get("crs")).toBe("EPSG:3857");
    expect(url.searchParams.get("transparent")).toBe("true");
    expect(url.searchParams.get("_")).toBe("5901234");
    expect(radarTiles(1)).toContain("bbox={bbox-epsg-3857}");
  });

  it("stamps a new value every five minutes", () => {
    const t = Date.parse("2026-11-22T21:00:00Z");
    expect(radarStamp(t + 4 * 60_000)).toBe(radarStamp(t));
    expect(radarStamp(t + 5 * 60_000)).toBe(radarStamp(t) + 1);
  });
});

describe("satelliteDay", () => {
  it("is the latest finished day: yesterday from 06:00 UTC, the day before until then", () => {
    expect(satelliteDay(Date.parse("2026-11-22T05:59:00Z"))).toBe("2026-11-20");
    expect(satelliteDay(Date.parse("2026-11-22T06:00:00Z"))).toBe("2026-11-21");
    expect(satelliteDay(Date.parse("2026-11-22T21:00:00Z"))).toBe("2026-11-21");
  });

  it("names the day in the GIBS tile address", () => {
    expect(satelliteTiles("2026-11-21")).toBe(
      "https://gibs.earthdata.nasa.gov/wmts/epsg3857/best/VIIRS_NOAA20_CorrectedReflectance_TrueColor/default/2026-11-21/GoogleMapsCompatible_Level9/{z}/{y}/{x}.jpg",
    );
  });
});
