import { describe, expect, it } from "vitest";
import { embedCode } from "./embed";
import { parse } from "./urlState";

const ORIGIN = "https://sentryatlas.com";
const src = (code: string) => code.match(/src="([^"]*)"/)?.[1];

describe("embedCode", () => {
  it("frames the whole globe for a link with no view", () => {
    expect(embedCode(ORIGIN, parse(""))).toBe(
      '<iframe src="https://sentryatlas.com/embed/" title="SentryAtlas map of current events" width="100%" height="450" style="border:0" loading="lazy"></iframe>',
    );
  });

  it("keeps the view, the types and the window, and leaves the selected event out", () => {
    const view = parse("?at=18.2,-66.5,6&types=weather,flood&time=30d&hours=36&event=usgs-x");
    expect(src(embedCode(ORIGIN, view))).toBe("https://sentryatlas.com/embed/?at=18.2,-66.5,6&amp;types=weather,flood&amp;time=30d&amp;hours=36");
  });

  it("keeps the layers that are on, so an embed can show radar", () => {
    expect(src(embedCode(ORIGIN, parse("?types=storm&layers=radar")))).toBe("https://sentryatlas.com/embed/?types=storm&amp;layers=radar");
  });

  it("writes every & as &amp;, so no part of the address reads as a character reference", () => {
    const code = embedCode(ORIGIN, parse("?types=flood&time=24h"));
    expect(code).not.toMatch(/&(?!amp;)/);
  });
});
