import { describe, expect, it } from "vitest";
import { normalize, prefixOf } from "./placeKey.mjs";
import { countryName, searchPlaces, type Place } from "./places";

describe("place keys", () => {
  it("folds accents, case and punctuation, and files by two letters", () => {
    expect(normalize("São Paulo")).toBe("saopaulo");
    expect(normalize("Saint-Étienne")).toBe("saintetienne");
    expect(prefixOf("Zürich")).toBe("zu");
    expect(prefixOf("A")).toBe("");
  });
});

describe("searchPlaces", () => {
  const sa: Place[] = [["São Paulo", "saopaulo", "BR", -46.6, -23.5], ["Santiago", "santiago", "CL", -70.6, -33.4], ["San Francisco", "sanfrancisco", "US", -122.4, 37.8]];
  const asked: string[] = [];
  const f = (async (url: string) => {
    asked.push(url);
    return new Response(JSON.stringify(url.endsWith("/sa.json") ? sa : []));
  }) as unknown as typeof fetch;

  it("finds places whose name starts like the query, from one file of this site", async () => {
    expect((await searchPlaces("San F", 8, f)).map((p) => p[0])).toEqual(["San Francisco"]);
    expect((await searchPlaces("sao", 8, f)).map((p) => p[0])).toEqual(["São Paulo"]);
    expect(asked.every((u) => u.startsWith("/data/places/"))).toBe(true);
  });

  it("asks for each file once", async () => {
    const before = asked.length;
    await searchPlaces("san", 8, f);
    await searchPlaces("sant", 8, f);
    expect(asked.length).toBe(before);
  });

  it("needs two letters", async () => {
    expect(await searchPlaces("s", 8, f)).toEqual([]);
  });

  it("names countries from the browser's data", () => {
    expect(countryName("BR")).toBe("Brazil");
  });
});
