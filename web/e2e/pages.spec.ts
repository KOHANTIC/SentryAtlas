import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { PAGES } from "../src/lib/pages";

for (const page of PAGES) {
  test(`${page.path} renders, in both themes, with no accessibility violations`, async ({ page: p }) => {
    for (const colorScheme of ["light", "dark"] as const) {
      await p.emulateMedia({ colorScheme });
      const res = await p.goto(page.path);
      expect(res?.status()).toBe(200);
      await expect(p.getByRole("heading", { level: 1 })).toBeVisible();
      const { violations } = await new AxeBuilder({ page: p }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
      expect(violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
    }
  });
}

test("the skip link moves focus to the content", async ({ page }) => {
  await page.goto("/");
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to content" })).toBeFocused();
});

test("an unknown path answers 404", async ({ page }) => {
  const res = await page.goto("/no-such-page/");
  expect(res?.status()).toBe(404);
});
