import { readFileSync } from "node:fs";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

// Playwright runs from web/.
const example = JSON.parse(readFileSync("../docs/api/v2/snapshot.example.json", "utf8"));
const quake = example.events[0];
const [qlon, qlat] = quake.location.center;

test.beforeEach(async ({ page }) => {
  // The example's own afternoon, so its events stay inside the time windows.
  await page.clock.setFixedTime(new Date("2026-11-22T21:00:00Z"));
  await page.route("**/api/v2/snapshot*", (r) => r.fulfill({ contentType: "application/json", body: JSON.stringify(example), headers: { ETag: '"example"' } }));
  await page.route("https://tiles.openfreemap.org/**", (r) => r.abort());
});

test("the embed opens on its link's view and filters, and credits the full map with the same view", async ({ page }) => {
  await page.goto("/embed/?at=18.2,-66.5,6&types=weather&time=30d");
  await expect(page.getByRole("status").first()).toHaveText("2 events, last 30 days");
  await expect(page.locator(".map-canvas")).toHaveAttribute("data-view", "18.20,-66.50,6.0");
  const credit = page.getByRole("link", { name: /SentryAtlas/ });
  await expect(credit).toHaveAttribute("href", "/map/?at=18.2,-66.5,6&types=weather&time=30d");
  await expect(credit).toHaveAttribute("target", "_blank");
});

test("the embed renders in both themes with no accessibility violations, and asks not to be indexed", async ({ page }) => {
  for (const colorScheme of ["light", "dark"] as const) {
    await page.emulateMedia({ colorScheme });
    await page.goto("/embed/");
    await expect(page.getByRole("status").first()).toContainText("events, last 7 days");
    const { violations } = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
    expect(violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  }
  await expect(page.locator('meta[name="robots"]')).toHaveAttribute("content", /noindex/);
});

test("a plain scroll over the embed leaves the map alone, so the page around it scrolls", async ({ page, isMobile }) => {
  test.skip(isMobile, "a wheel is a desktop gesture");
  await page.goto("/embed/?at=18.2,-66.5,6");
  const canvas = page.locator(".map-canvas");
  await expect(canvas).toHaveAttribute("data-view", "18.20,-66.50,6.0");
  const box = await canvas.boundingBox();
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await page.mouse.wheel(0, -600);
  await page.waitForTimeout(1000);
  await expect(canvas).toHaveAttribute("data-view", "18.20,-66.50,6.0");
});

test("a click on an event shows a card that links to the full map open on it", async ({ page }) => {
  await page.goto(`/embed/?at=${qlat},${qlon},6`);
  await expect(page.getByRole("status").first()).toContainText("events, last 7 days");
  const card = page.getByRole("heading", { level: 2, name: "1 event here" });
  // The quake sits at the frame's center; the map draws its data a moment after it arrives.
  await expect(async () => {
    await page.locator(".map-canvas").click();
    await expect(card).toBeVisible({ timeout: 1000 });
  }).toPass();
  await expect(card).toBeFocused();
  const link = page.getByRole("link", { name: new RegExp(quake.title) });
  await expect(link).toHaveAttribute("href", `/map/?at=${qlat},${qlon},6&event=${quake.id}`);
  await expect(link).toHaveAttribute("target", "_blank");
  await page.keyboard.press("Escape");
  await expect(card).toBeHidden();
});

test("without WebGL2 the embed says so and keeps its credit", async ({ page }) => {
  await page.addInitScript(() => {
    const get = HTMLCanvasElement.prototype.getContext;
    // @ts-expect-error: narrowing the overloads is not the point here
    HTMLCanvasElement.prototype.getContext = function (kind: string, ...rest: unknown[]) {
      return kind === "webgl2" ? null : get.call(this, kind, ...rest);
    };
  });
  await page.goto("/embed/");
  await expect(page.getByText("This browser cannot draw the map: it needs WebGL2.", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: /SentryAtlas/ })).toHaveAttribute("href", "/map/");
});
