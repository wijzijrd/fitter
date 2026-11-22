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

test.describe("phone, touch targets", () => {
  test.use({ viewport: { width: 320, height: 720 }, hasTouch: true, isMobile: true });

  for (const [route, selector] of [
    ["/campaigns/new", ".back, .navbar .wordmark"],
    ["/campaigns/c1", ".back, .navbar .wordmark, [role=tab]"],
  ] as const) {
    test(`back link, wordmark and tabs are 44px on ${route}`, async ({ page }) => {
      await page.goto(route);
      await expect(page.locator(".back")).toBeVisible();
      const sizes = await page.locator(selector).evaluateAll((els) => els.map((e) => { const b = e.getBoundingClientRect(); return [e.textContent?.trim(), Math.round(b.width), Math.round(b.height)]; }));
      for (const [, w, h] of sizes) expect.soft(Math.min(w as number, h as number), JSON.stringify(sizes)).toBeGreaterThanOrEqual(44);
    });
  }
});
