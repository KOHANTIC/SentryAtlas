import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";

// Playwright runs from web/.
const example = readFileSync("../docs/api/v2/snapshot.example.json", "utf8");

test.beforeEach(async ({ page }) => {
  // The example snapshot instead of the live API, and no basemap tiles: the checks are about the page.
  await page.route("**/api/v2/snapshot*", (r) => r.fulfill({ contentType: "application/json", body: example, headers: { ETag: '"example"' } }));
  await page.route("https://tiles.openfreemap.org/**", (r) => r.abort());
});

test("the map page shows the key for all 15 types and the five severity levels", async ({ page }) => {
  await page.goto("/map/");
  const key = page.getByRole("region", { name: "Key" }).or(page.locator("section.legend"));
  await expect(key.locator(".legend-types li")).toHaveCount(15);
  await expect(key.locator(".legend-levels li")).toHaveText(["Extreme", "Severe", "Moderate", "Minor", "Not graded"]);
});

test("the map page counts the placed and unplaced events it holds", async ({ page }) => {
  await page.goto("/map/");
  await expect(page.getByRole("status").first()).toHaveText("5 events, last 7 days");
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
  await expect(page.getByText("This browser cannot draw the map: it needs WebGL2.")).toBeVisible();
  await expect(page.locator("section.legend")).toBeVisible();
});
