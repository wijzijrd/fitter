import { test, expect, focused, overflowsX } from "./api";

test.use({ viewport: { width: 320, height: 760 }, hasTouch: true, isMobile: true });

test.beforeEach(async ({ page }) => {
  await page.goto("/activities/new");
  await expect(page.locator(".history b", { hasText: "Push day" })).toBeVisible();
});

test("submitting empty focuses the first invalid field", async ({ page }) => {
  await page.locator('button[type="submit"]').click();
  expect((await focused(page)).id).toBe("al-sport");
});

test("Time is optional", async ({ page }) => {
  await page.selectOption("#al-sport", "Run");
  await page.locator('button[type="submit"]').click();
  await expect(page.locator("#al-duration")).not.toHaveAttribute("aria-invalid", "true");
  await expect(page.locator("#al-duration-error")).toBeEmpty();
});

test("lift rows stay usable at 320px", async ({ page }) => {
  await page.selectOption("#al-sport", "WeightTraining");
  await page.getByRole("button", { name: "Add lift" }).click();
  const width = await page.locator(".lift input").first().evaluate((e) => e.getBoundingClientRect().width);
  expect(width).toBeGreaterThan(200);
  expect(await overflowsX(page)).toBe(false);
});

test("delete asks first and keeps focus in the list", async ({ page }) => {
  const history = page.locator(".history");
  await history.locator("[data-delete]").first().click();
  expect((await focused(page)).text).toBe("Keep");

  await page.keyboard.press("Enter");
  expect((await focused(page)).text).toContain("Push day");

  await history.locator("[data-delete]").first().click();
  await history.locator(".confirm .pill--primary").click();
  await expect(page.locator(".history b", { hasText: "Push day" })).toBeHidden();
  expect((await focused(page)).text).toContain("Pull day");
});
