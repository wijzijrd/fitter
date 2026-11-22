import { test, expect, overflowsX } from "./api";

test.use({ viewport: { width: 320, height: 760 }, hasTouch: true, isMobile: true });

test.describe("first visit after a release", () => {
  test.use({ unseenSplash: true });
  // Pin the clock inside the release's first week so these don't rot as real time passes.
  test.beforeEach(async ({ page }) => page.clock.setFixedTime(new Date("2026-11-23T09:00:00Z")));

  test("shows once, traps focus, closes on Escape and stays closed", async ({ page }) => {
    await page.goto("/");
    const dialog = page.getByRole("dialog", { name: "What's new" });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("listitem")).toHaveCount(8);
    await expect(dialog.getByRole("button", { name: "Got it" })).toBeFocused();
    expect(await overflowsX(page)).toBe(false);

    // The page behind a modal dialog is inert: Tab never lands on the navbar or the campaigns list.
    for (let i = 0; i < 4; i++) {
      await page.keyboard.press("Tab");
      expect(await page.evaluate(() => { const a = document.activeElement; return a === document.body || !!a?.closest("dialog"); })).toBe(true);
    }
    const saved = page.waitForRequest((r) => r.url().endsWith("MarkWhatsNewSeen"));
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    expect((await saved).postDataJSON().release).toMatch(/\d{4}-\d{2}-\d{2}/);

    await page.reload();
    await expect(page.getByRole("heading", { name: "Campaigns" })).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });

  test("a release older than a week is no longer offered on its own", async ({ page }) => {
    await page.clock.setFixedTime(new Date("2026-12-01T09:00:00Z"));
    await page.goto("/settings");
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await page.getByRole("button", { name: "Show what's new" }).click();
    await expect(page.getByRole("dialog", { name: "What's new" })).toBeVisible();
  });

  test("Got it closes it", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Got it" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });

  test("fits a short 320px screen and its button is 44px tall", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 568 });
    await page.goto("/");
    const btn = page.getByRole("button", { name: "Got it" });
    await expect(btn).toBeVisible();
    expect((await btn.boundingBox())!.height).toBeGreaterThanOrEqual(43.5);
    expect(await overflowsX(page)).toBe(false);
  });
});

test("a seen release stays quiet, and Settings reopens it", async ({ page }) => {
  await page.goto("/settings");
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const open = page.getByRole("button", { name: "Show what's new" });
  await open.click();
  await expect(page.getByRole("dialog", { name: "What's new" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(open).toBeFocused();
});
