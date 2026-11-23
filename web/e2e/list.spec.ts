import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";

// Playwright runs from web/.
const example = JSON.parse(readFileSync("../docs/api/v2/snapshot.example.json", "utf8"));

test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-11-22T21:00:00Z"));
  await page.route("**/api/v2/snapshot*", (r) => r.fulfill({ contentType: "application/json", body: JSON.stringify(example) }));
  await page.route("**/api/v2/events/**", (r) => {
    const id = decodeURIComponent(new URL(r.request().url()).pathname.split("/").pop() ?? "");
    const e = example.events.find((x: { id: string }) => x.id === id);
    return e ? r.fulfill({ contentType: "application/json", body: JSON.stringify({ event: e, source: example.sources[0] }) }) : r.fulfill({ status: 404, body: "{}" });
  });
  // No WebGL2: the list then holds every event, whatever the globe would show.
  await page.addInitScript(() => {
    const get = HTMLCanvasElement.prototype.getContext;
    // @ts-expect-error: narrowing the overloads is not the point here
    HTMLCanvasElement.prototype.getContext = function (kind: string, ...rest: unknown[]) {
      return kind === "webgl2" ? null : get.call(this, kind, ...rest);
    };
  });
});

const rows = (page: import("@playwright/test").Page) => page.getByRole("option");

test("the list holds the window's events, newest first or most severe first", async ({ page }) => {
  await page.goto("/map/?time=30d");
  await expect(rows(page)).toHaveCount(5);
  // Newest: the two NWS alerts of 22 November lead.
  await expect(rows(page).first()).toContainText("NWS");
  await page.getByRole("button", { name: "Most severe" }).click();
  await expect(page.getByRole("button", { name: "Most severe" })).toHaveAttribute("aria-pressed", "true");
  await expect(rows(page)).toHaveCount(5);
});

test("the keyboard walks the list and Enter opens the event", async ({ page }) => {
  await page.goto("/map/?time=30d");
  const list = page.getByRole("listbox", { name: "Events in view" });
  await list.focus();
  await page.keyboard.press("ArrowDown");
  await expect(list).toHaveAttribute("aria-activedescendant", "ev-1");
  const title = (await rows(page).nth(1).locator(".row-title").textContent()) ?? "";
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { level: 2, name: title })).toBeVisible();
  await expect(rows(page).nth(1)).toHaveAttribute("aria-selected", "true");
});

test("a selection from a link marks its row", async ({ page }) => {
  const quake = example.events[0];
  await page.goto(`/map/?time=30d&event=${encodeURIComponent(quake.id)}`);
  await expect(page.getByRole("option", { selected: true })).toContainText(quake.title);
});
