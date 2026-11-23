import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";

// Playwright runs from web/.
const example = JSON.parse(readFileSync("../docs/api/v2/snapshot.example.json", "utf8"));
const quake = example.events[0];
const nws = example.events[2];

test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-11-22T21:00:00Z"));
  await page.route("**/api/v2/snapshot*", (r) => r.fulfill({ contentType: "application/json", body: JSON.stringify(example) }));
  await page.route("**/api/v2/events/**", (r) => {
    const id = decodeURIComponent(new URL(r.request().url()).pathname.split("/").pop() ?? "");
    const e = example.events.find((x: { id: string }) => x.id === id);
    if (!e) return r.fulfill({ status: 404, contentType: "application/json", body: '{"error":"no event with that id is being served"}' });
    const full = id === nws.id ? { ...e, description: "Flooding of low roads is possible.", details: { noaa: { ...e.details.noaa, instruction: "Turn around, don't drown." } } } : e;
    return r.fulfill({ contentType: "application/json", body: JSON.stringify({ event: full, source: example.sources[0] }) });
  });
  await page.route("https://tiles.openfreemap.org/**", (r) => r.abort());
});

test("a link to an event opens the panel on it, with focus on its title", async ({ page }) => {
  await page.goto(`/map/?event=${encodeURIComponent(quake.id)}`);
  const title = page.getByRole("heading", { level: 2, name: quake.title });
  await expect(title).toBeVisible();
  await expect(title).toBeFocused();
  await expect(page.locator(".panel .facts")).toContainText("Magnitude");
  await expect(page.locator(".panel .facts")).toContainText("UTC");
});

test("an NWS alert shows its own text and what to do, word for word", async ({ page }) => {
  await page.goto(`/map/?event=${encodeURIComponent(nws.id)}`);
  await expect(page.locator(".panel .do")).toContainText("Turn around, don't drown.");
  await expect(page.locator(".panel")).toContainText("Flooding of low roads is possible.");
});

test("an event no longer in its feed says it has ended", async ({ page }) => {
  await page.goto("/map/?event=usgs-gone");
  await expect(page.getByRole("heading", { level: 2, name: "This event has ended" })).toBeVisible();
});

test("Close and Escape close the panel and take the event out of the link", async ({ page }) => {
  for (const how of ["button", "escape"] as const) {
    await page.goto(`/map/?event=${encodeURIComponent(quake.id)}`);
    await expect(page.locator(".panel")).toBeVisible();
    if (how === "button") await page.getByRole("button", { name: "Close" }).click();
    else await page.keyboard.press("Escape");
    await expect(page.locator(".panel")).toHaveCount(0);
    expect(new URL(page.url()).searchParams.get("event")).toBeNull();
  }
});

test("on a phone the sheet changes height by buttons, with no drag", async ({ page, isMobile }) => {
  test.skip(!isMobile, "the sheet is the phone layout");
  await page.goto(`/map/?event=${encodeURIComponent(quake.id)}`);
  const panel = page.locator(".panel");
  await expect(panel).toHaveAttribute("data-height", "half");
  await page.getByRole("button", { name: "More" }).click();
  await expect(panel).toHaveAttribute("data-height", "full");
  await page.getByRole("button", { name: "Map" }).click();
  await expect(panel).toHaveAttribute("data-height", "peek");
  await page.getByRole("button", { name: "More" }).click();
  await expect(panel).toHaveAttribute("data-height", "half");
});
