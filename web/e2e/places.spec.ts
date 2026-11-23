import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";

// Playwright runs from web/.
const example = readFileSync("../docs/api/v2/snapshot.example.json", "utf8");

test("typing a city offers it, the keyboard chooses it, and the map's link goes there", async ({ page, baseURL }) => {
  await page.route("**/api/v2/snapshot*", (r) => r.fulfill({ contentType: "application/json", body: example }));
  await page.route("https://tiles.openfreemap.org/**", (r) => r.abort());
  await page.goto("/map/");
  const asked: string[] = [];
  page.on("request", (r) => asked.push(r.url()));

  const box = page.getByRole("combobox", { name: "Go to a place" });
  await box.fill("san franc");
  await expect(page.getByRole("option").first()).toContainText("San Francisco");
  await expect(page.getByRole("option").first()).toContainText("United States");
  // Down highlights the first suggestion; Enter chooses it.
  await box.press("ArrowDown");
  await expect(box).toHaveAttribute("aria-activedescendant", /-0$/);
  await box.press("Enter");
  await expect(box).toHaveValue("San Francisco");
  await expect.poll(() => new URL(page.url()).searchParams.get("at")).toMatch(/^37\.7\d*,-122\.4\d*,8$/);

  await expect(page.locator(".maplibregl-ctrl-attrib-inner")).toContainText("Places: GeoNames");

  // Nothing typed went to another site.
  const origin = new URL(baseURL!).origin;
  expect(asked.filter((u) => /\/data\/places\//.test(u) && !u.startsWith(origin))).toEqual([]);
  expect(asked.filter((u) => /geonames/i.test(u))).toEqual([]);
});
