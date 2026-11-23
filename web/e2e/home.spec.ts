import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";

// Playwright runs from web/.
const example = readFileSync("../docs/api/v2/snapshot.example.json", "utf8");

test("the home page shows today's figures from the snapshot and offers the map and the waiting list", async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-11-22T21:00:00Z"));
  await page.route("**/api/v2/snapshot*", (r) => r.fulfill({ contentType: "application/json", body: example }));
  await page.route("https://tiles.openfreemap.org/**", (r) => r.abort());
  await page.goto("/");
  const figures = page.locator(".figures");
  // Two NWS alerts began in the last 24 hours; one quake and two NWS alerts in all.
  await expect(figures).toContainText("2events in the last 24 hours");
  await expect(figures).toContainText("2US weather alerts in force");
  await expect(figures).toContainText("1earthquakes in the last 7 days");
  await expect(page.getByRole("link", { name: "Open the map" })).toHaveAttribute("href", "/map/");
  await expect(page.getByRole("button", { name: "Join the waiting list" })).toBeVisible();
});

test("the sitemap lists every page at its address after the cutover", async ({ request }) => {
  const xml = await (await request.get("/sitemap.xml")).text();
  for (const path of ["/", "/map/", "/data/", "/developers/", "/privacy/", "/terms/", "/alerts/"]) {
    expect(xml).toContain(`<loc>https://sentryatlas.com${path}</loc>`);
  }
});
