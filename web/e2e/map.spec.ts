import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";

// Playwright runs from web/.
const example = readFileSync("../docs/api/v2/snapshot.example.json", "utf8");

test.beforeEach(async ({ page }) => {
  // The example's own afternoon, so its events stay inside the time windows.
  await page.clock.setFixedTime(new Date("2026-11-22T21:00:00Z"));
  // The example snapshot instead of the live API, and no basemap tiles: the checks are about the page.
  await page.route("**/api/v2/snapshot*", (r) => r.fulfill({ contentType: "application/json", body: example, headers: { ETag: '"example"' } }));
  await page.route("https://tiles.openfreemap.org/**", (r) => r.abort());
});

test("the map page shows the key for all 15 types and the five severity levels", async ({ page }) => {
  await page.goto("/map/");
  await page.getByRole("tab", { name: "Key" }).click();
  const key = page.getByRole("region", { name: "Key" }).or(page.locator("section.legend"));
  await expect(key.locator(".legend-types li")).toHaveCount(15);
  await expect(key.locator(".legend-levels li")).toHaveText(["Extreme", "Severe", "Moderate", "Minor", "Not graded"]);
});

test("the map page counts the events in the time window, placed or not", async ({ page }) => {
  await page.goto("/map/");
  // The cyclone began on 1 November, outside the last 7 days.
  await expect(page.getByRole("status").first()).toContainText("of 4 events, last 7 days");
});

test("a link reproduces its filters and time window, and loading does not rewrite it", async ({ page }) => {
  const link = "/map/?at=18.2,-66.5,6&types=weather&time=30d";
  await page.goto(link);
  await expect(page.getByRole("status").first()).toContainText("of 2 events, last 30 days");
  // The map opens on the link's view, not on the whole globe.
  await expect(page.locator(".map-canvas")).toHaveAttribute("data-view", "18.20,-66.50,6.0");
  await page.waitForTimeout(1000);
  expect(new URL(page.url()).pathname + new URL(page.url()).search).toBe(link);
});

test("Copy link puts the page's address on the clipboard", async ({ page, context, browserName }) => {
  test.skip(browserName !== "chromium", "clipboard permissions are Chromium's");
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/map/?types=flood");
  await page.getByRole("button", { name: "Copy link" }).click();
  await expect(page.getByText("Link copied")).toBeVisible();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(page.url());
});

test("Embed gives the code for the map's view, types and window, and copies it", async ({ page, context, browserName }) => {
  test.skip(browserName !== "chromium", "clipboard permissions are Chromium's");
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/map/?at=18.2,-66.5,6&types=weather&time=30d");
  const embed = page.getByRole("button", { name: "Embed" });
  await expect(embed).toHaveAttribute("aria-expanded", "false");
  await embed.click();
  await expect(embed).toHaveAttribute("aria-expanded", "true");
  const origin = new URL(page.url()).origin;
  const code = page.getByRole("textbox", { name: "Embed code" });
  await expect(code).toHaveValue(
    `<iframe src="${origin}/embed/?at=18.2,-66.5,6&amp;types=weather&amp;time=30d" title="SentryAtlas map of current events" width="100%" height="450" style="border:0" loading="lazy"></iframe>`,
  );
  await page.getByRole("button", { name: "Copy code" }).click();
  await expect(page.getByText("Code copied")).toBeVisible();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(await code.inputValue());
});

test("without WebGL2 the page says so instead of drawing", async ({ page }) => {
  await page.addInitScript(() => {
    const get = HTMLCanvasElement.prototype.getContext;
    // @ts-expect-error: narrowing the overloads is not the point here
    HTMLCanvasElement.prototype.getContext = function (kind: string, ...rest: unknown[]) {
      return kind === "webgl2" ? null : get.call(this, kind, ...rest);
    };
  });
  await page.goto("/map/");
  await expect(page.getByText("This browser cannot draw the map: it needs WebGL2. The list has every event.")).toBeVisible();
  await expect(page.getByRole("listbox", { name: "Events in view" })).toBeVisible();
});
