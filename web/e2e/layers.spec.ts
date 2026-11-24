import { readFileSync } from "node:fs";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page, type Request } from "@playwright/test";

// Playwright runs from web/.
const example = readFileSync("../docs/api/v2/snapshot.example.json", "utf8");
// A clear 1×1 picture stands in for every radar and satellite tile, so nothing leaves the test.
const PIXEL = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=", "base64");
const RADAR = "https://nowcoast.noaa.gov/";
const SATELLITE = "https://gibs.earthdata.nasa.gov/";

test.beforeEach(async ({ page }) => {
  // The example's own afternoon: the latest full day of imagery is then 21 November.
  await page.clock.setFixedTime(new Date("2026-11-22T21:00:00Z"));
  await page.route("**/api/v2/snapshot*", (r) => r.fulfill({ contentType: "application/json", body: example, headers: { ETag: '"example"' } }));
  await page.route("https://tiles.openfreemap.org/**", (r) => r.abort());
  for (const host of [RADAR, SATELLITE]) await page.route(`${host}**`, (r) => r.fulfill({ contentType: "image/png", body: PIXEL }));
});

const asked = (host: string) => (r: Request) => r.url().startsWith(host) && !r.url().includes("{");
const ready = (page: Page) => expect(page.locator(".map-canvas")).toHaveAttribute("data-view", /./);

test("with the layers off, the page never asks NOAA or NASA for imagery", async ({ page }) => {
  const imagery: string[] = [];
  page.on("request", (r) => {
    if (r.url().startsWith(RADAR) || r.url().startsWith(SATELLITE)) imagery.push(r.url());
  });
  await page.goto("/map/");
  await ready(page);
  await expect(page.getByRole("status").first()).toContainText("events, last 7 days");
  await page.waitForTimeout(1500);
  expect(imagery).toEqual([]);
});

test("Radar turns on NOAA's radar, says what it shows, and goes in and out of the link", async ({ page }) => {
  await page.goto("/map/");
  await ready(page);
  const radar = page.getByRole("button", { name: "Radar (US)" });
  await expect(radar).toHaveAttribute("aria-pressed", "false");
  const tile = page.waitForRequest((r) => asked(RADAR)(r) && /bbox=-?\d/.test(r.url()));
  await radar.click();
  await expect(radar).toHaveAttribute("aria-pressed", "true");
  expect(new URL(page.url()).searchParams.get("layers")).toBe("radar");
  await tile;
  await expect(page.getByText(/^Radar: NOAA's latest picture/)).toBeVisible();
  await radar.click();
  await expect(radar).toHaveAttribute("aria-pressed", "false");
  expect(new URL(page.url()).searchParams.has("layers")).toBe(false);
  await expect(page.getByText(/^Radar: NOAA's latest picture/)).toBeHidden();
});

test("Satellite asks NASA for the latest full day and names it", async ({ page }) => {
  await page.goto("/map/");
  await ready(page);
  const tile = page.waitForRequest((r) => asked(SATELLITE)(r) && r.url().includes("/VIIRS_NOAA20_CorrectedReflectance_TrueColor/default/2026-11-21/"));
  await page.getByRole("button", { name: "Satellite" }).click();
  await tile;
  await expect(page.getByText("Satellite: NASA's true-colour picture of 21 Nov, from daylight passes.")).toBeVisible();
  expect(new URL(page.url()).searchParams.get("layers")).toBe("satellite");
});

test("an embed whose link has radar shows it", async ({ page }) => {
  const tile = page.waitForRequest(asked(RADAR));
  await page.goto("/embed/?layers=radar");
  await tile;
});

test("the map with both layers on has no accessibility violations, in both themes", async ({ page }) => {
  for (const colorScheme of ["light", "dark"] as const) {
    await page.emulateMedia({ colorScheme });
    await page.goto("/map/?layers=radar,satellite");
    await expect(page.getByText(/^Satellite: NASA's/)).toBeVisible();
    const { violations } = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
    expect(violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  }
});
