import { test, expect, overflowsX } from "./api";
import { CURRENT, releases, unseenReleases } from "../../src/lib/releases";

test.use({ viewport: { width: 320, height: 760 }, hasTouch: true, isMobile: true });

const versionHeadings = (scope: import("@playwright/test").Locator) => scope.getByRole("heading", { level: 3 });

test.describe("a member who has never closed it", () => {
  test.use({ seen: "" });

  test("pops up with the newest feature release, traps focus, saves on Escape and stays closed", async ({ page }) => {
    await page.goto("/");
    const dialog = page.getByRole("dialog", { name: "Release notes" });
    await expect(dialog).toBeVisible();
    await expect(versionHeadings(dialog)).toHaveText(unseenReleases("").map((r) => r.version));
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
    expect((await saved).postDataJSON().release).toBe(CURRENT);

    await page.reload();
    await expect(page.getByRole("heading", { name: "Campaigns" })).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });

  test("Got it closes it", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Got it" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  });

  test("All release notes closes it and opens the page", async ({ page }) => {
    await page.goto("/");
    const saved = page.waitForRequest((r) => r.url().endsWith("MarkWhatsNewSeen"));
    await page.getByRole("dialog").getByRole("link", { name: "All release notes" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(page).toHaveURL(/\/settings\/release-notes$/);
    await expect(page.getByRole("heading", { name: "Release notes", level: 1 })).toBeVisible();
    expect((await saved).postDataJSON().release).toBe(CURRENT);
  });

  test("fits a short 320px screen and its button is 44px tall", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 568 });
    await page.goto("/");
    const btn = page.getByRole("button", { name: "Got it" });
    await expect(btn).toBeVisible();
    expect((await btn.boundingBox())!.height).toBeGreaterThanOrEqual(43.5);
    expect(await overflowsX(page)).toBe(false);
  });

  test("opens at the top on a short screen, with Got it always in view", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 568 });
    await page.goto("/");
    const dialog = page.getByRole("dialog", { name: "Release notes" });
    const btn = dialog.getByRole("button", { name: "Got it" });
    await expect(btn).toBeFocused();
    await expect(dialog.getByRole("heading", { name: "Release notes" })).toBeInViewport({ ratio: 1 });
    await expect(dialog.locator("li").first()).toBeInViewport();
    await expect(btn).toBeInViewport({ ratio: 1 });
    // Scrolling the list to its end leaves the heading and button where they were.
    await dialog.locator(".items").evaluate((el) => (el.scrollTop = el.scrollHeight));
    await expect(dialog.locator("li").last()).toBeInViewport();
    await expect(dialog.getByRole("heading", { name: "Release notes" })).toBeInViewport({ ratio: 1 });
    await expect(btn).toBeInViewport({ ratio: 1 });
  });
});

test.describe("a member who closed the 22 Nov What's new", () => {
  test.use({ seen: "2026-11-22" });

  test("sees only the releases after it", async ({ page }) => {
    await page.goto("/");
    const dialog = page.getByRole("dialog", { name: "Release notes" });
    await expect(versionHeadings(dialog)).toHaveText(unseenReleases("2026-11-22").map((r) => r.version));
  });
});

test("nothing new stays quiet, and the page lists every release", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Campaigns" })).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  await page.goto("/settings/release-notes");
  await expect(page.getByRole("heading", { name: "Release notes", level: 1 })).toBeVisible();
  await expect(page.getByText(`You're on version ${CURRENT}.`)).toBeVisible();
  await expect(page.getByRole("heading", { level: 2 })).toHaveText(releases.map((r) => r.version));
  await expect(page.getByRole("region", { name: "0.7.1" })).toContainText("Fixes and polish");
  expect(await overflowsX(page)).toBe(false);
});
