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

test("a zone-only alert loads the zone outlines to draw its area", async ({ page }) => {
  const zoned = example.events[3];
  const zones = page.waitForRequest((r) => r.url().endsWith("/data/nws-zones.json"));
  await page.goto(`/map/?time=30d&event=${encodeURIComponent(zoned.id)}`);
  await zones;
});

test("a tsunami warning shows the center's instructions and the earthquake it is about", async ({ page }) => {
  const warning = {
    id: "tsunami-PHEB-26327000",
    source: "tsunami",
    type: "tsunami",
    title: "Tsunami Warning: VANUATU REGION",
    severity: { level: "extreme", source_level: "Tsunami Warning" },
    time: { started: "2026-11-22T20:40:57Z", updated: "2026-11-22T20:49:16Z", expires: "2026-11-22T23:49:16Z" },
    location: { center: [172.279, -13.546], geometry: { type: "Point", coordinates: [172.279, -13.546] } },
    url: "https://www.tsunami.gov/events/PHEB/2026/11/22/26327000/2/WEGM42/WEGM42.txt",
    details: {
      tsunami: {
        center: "PTWC",
        event: "Tsunami Warning",
        urgency: "Immediate",
        certainty: "Likely",
        message_type: "Alert",
        area: "VANUATU REGION",
        quake: { magnitude: 6.5, magnitude_type: "Mwp", depth_km: 614, origin: "2026-11-22T20:40:57Z" },
      },
    },
  };
  const full = { ...warning, details: { tsunami: { ...warning.details.tsunami, instruction: "Move to high ground now." } } };
  await page.route("**/api/v2/snapshot*", (r) => r.fulfill({ contentType: "application/json", body: JSON.stringify({ ...example, events: [...example.events, warning] }) }));
  await page.route(`**/api/v2/events/${warning.id}`, (r) => r.fulfill({ contentType: "application/json", body: JSON.stringify({ event: full, source: example.sources[0] }) }));
  await page.goto(`/map/?event=${warning.id}`);
  await expect(page.getByRole("heading", { level: 2, name: warning.title })).toBeVisible();
  await expect(page.locator(".panel .do")).toContainText("Move to high ground now.");
  await expect(page.locator(".panel .facts")).toContainText("M 6.5 (Mwp), 614 km deep");
  await expect(page.getByRole("link", { name: "Read the bulletin on tsunami.gov" })).toHaveAttribute("href", warning.url);
});
