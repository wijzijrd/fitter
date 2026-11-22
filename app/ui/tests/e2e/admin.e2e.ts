import { test, expect, focused, overflowsX } from "./api";

test.use({ viewport: { width: 375, height: 900 }, hasTouch: true, isMobile: true });

test.beforeEach(async ({ page }) => {
  await page.goto("/admin");
  await expect(page.getByText("Sam Okafor")).toBeVisible();
});

test("tabs move with arrow keys", async ({ page }) => {
  await page.locator("#admin-tab-users").focus();
  await page.keyboard.press("ArrowRight");
  expect((await focused(page)).id).toBe("admin-tab-catalog");
  await expect(page.locator("#admin-tab-catalog")).toHaveAttribute("aria-selected", "true");
  await page.keyboard.press("End");
  expect((await focused(page)).id).toBe("admin-tab-audit");
});

test("catalog delete asks first and keeps focus in the list", async ({ page }) => {
  await page.locator("#admin-tab-catalog").click();
  await expect(page.locator(".rows .name", { hasText: "Powerlifting" })).toBeVisible();
  expect(await overflowsX(page)).toBe(false);

  await page.locator("[data-delete-sport]").first().click();
  expect((await focused(page)).text).toBe("Keep");
  await page.locator(".confirm .pill--primary").click();
  await expect(page.locator(".rows .name", { hasText: "Powerlifting" })).toBeHidden();
  expect((await focused(page)).text).toContain("Climbing");
});

test("ban form takes focus and gives it back on Cancel", async ({ page }) => {
  await page.locator('[data-ban="u2"]').click();
  expect((await focused(page)).id).toBe("ban-u2");
  await page.getByRole("button", { name: "Cancel" }).click();
  expect((await focused(page)).text).toBe("Ban…");
});
