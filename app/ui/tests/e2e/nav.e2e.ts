import { test, expect, focused, overflowsX } from "./api";

test.describe("tablet, touch", () => {
  test.use({ viewport: { width: 768, height: 600 }, hasTouch: true, isMobile: true });

  test("inline nav links and Log out are 44px touch targets", async ({ page }) => {
    await page.goto("/activities/new");
    await expect(page.locator("h1")).toBeVisible();
    const heights = await page.locator(".navbar nav a, .navbar .logout").evaluateAll((els) => els.map((e) => e.getBoundingClientRect().height));
    expect(heights.length).toBeGreaterThan(0);
    for (const h of heights) expect(h).toBeGreaterThanOrEqual(44);
    expect(await overflowsX(page)).toBe(false);
  });
});

for (const width of [721, 760]) {
  test(`nav at ${width}px keeps links on one line with a long name`, async ({ page }) => {
    await page.setViewportSize({ width, height: 400 });
    await page.goto("/activities/new");
    await expect(page.locator("h1")).toBeVisible();
    const heights = await page.locator(".navbar nav a").evaluateAll((els) => els.map((e) => e.getBoundingClientRect().height));
    for (const h of heights) expect(h).toBeLessThan(30);
    expect(await overflowsX(page)).toBe(false);
  });
}

test.describe("phone", () => {
  test.use({ viewport: { width: 320, height: 760 }, hasTouch: true, isMobile: true });

  test("menu closes on Escape and returns focus to the button", async ({ page }) => {
    await page.goto("/activities/new");
    await page.locator(".menu-btn").click();
    await expect(page.locator("#site-nav")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.locator("#site-nav")).toBeHidden();
    expect((await focused(page)).text).toContain("Open menu");
  });
});
