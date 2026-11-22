import { test, expect, overflowsX } from "./api";

test.use({ viewport: { width: 390, height: 900 }, hasTouch: true, isMobile: true });

async function openFeed(page: import("@playwright/test").Page): Promise<void> {
  await page.goto("/campaigns/c1");
  await page.getByRole("tab", { name: "Feed" }).click();
  await expect(page.locator(".activity").first()).toBeVisible();
}

test("the feed shows the last week, what's new, and the viewer's own rows marked", async ({ page }) => {
  await openFeed(page);
  await expect(page.locator(".activity")).toHaveCount(3); // the 10-day-old ride is cut
  await expect(page.locator(".pulse")).toHaveText("Jo Park and Sam Okafor logged 2 activities since you last looked.");
  await expect(page.locator(".tear")).toContainText("2 new since you looked");
  await expect(page.locator(".activity.mine")).toHaveCount(1);
  await expect(page.locator(".activity.mine b")).toHaveText("You");
  expect(await overflowsX(page)).toBe(false);
});

test("rows carry the chosen details and a short list of lifts", async ({ page }) => {
  await openFeed(page);
  await expect(page.locator(".activity", { hasText: "Morning run" })).toContainText("5:12 /km · Effort 42");
  await expect(page.locator(".activity", { hasText: "Evening ride" })).toContainText("30.0 km/h · 212 W");
  const leg = page.locator(".activity", { hasText: "Leg day" });
  await expect(leg.locator(".lift-line")).toHaveText(["Squat 60 kg × 8", "Leg press 60 kg × 8", "Lunge 60 kg × 8", "+1 more"]);
  await expect(leg.locator(".val")).not.toContainText("0 km"); // no distance: its time shows instead
  await expect(page.locator("body")).not.toContainText(/heart/i);
});

test("leaving the tab marks the feed seen, and coming back reads caught up", async ({ page }) => {
  await openFeed(page);
  const marked = page.waitForRequest((r) => r.url().endsWith("MarkFeedSeen"));
  await page.getByRole("tab", { name: "Standings" }).click();
  await marked;
  await page.getByRole("tab", { name: "Feed" }).click();
  await expect(page.locator(".pulse")).toHaveText("You're caught up.");
  await expect(page.locator(".tear")).toHaveCount(0);
});

test("Settings turns details off, saves each choice, and the feed follows", async ({ page }) => {
  await page.goto("/campaigns/c1");
  // The route change that finishes loading closes the menu, so open it only once the page is up.
  await expect(page.getByRole("tab", { name: "Feed" })).toBeVisible();
  await page.locator(".menu-btn").click();
  await page.getByRole("link", { name: /^Settings, signed in as/ }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  for (const box of await page.getByRole("checkbox").all()) {
    await expect(box).toBeChecked();
    expect((await box.locator("xpath=ancestor::label").boundingBox())!.height).toBeGreaterThanOrEqual(43.5);
  }
  const saved = page.waitForRequest((r) => r.url().endsWith("UpdateFeedSettings"));
  await page.getByRole("checkbox", { name: /Pace and speed/ }).uncheck();
  expect((await saved).postDataJSON().feedSettings).toEqual({ showPower: true, showEffort: true, showLifts: true }); // JSON leaves out false
  await page.getByRole("checkbox", { name: /Exercises/ }).uncheck();
  expect(await overflowsX(page)).toBe(false);

  await page.goto("/campaigns/c1");
  await page.getByRole("tab", { name: "Feed" }).click();
  await expect(page.locator(".activity", { hasText: "Morning run" })).not.toContainText("/km");
  await expect(page.locator(".activity", { hasText: "Morning run" })).toContainText("Effort 42");
  await expect(page.locator(".lift-line")).toHaveCount(0);
  await page.getByRole("link", { name: "Choose which details show" }).click();
  await expect(page).toHaveURL(/\/settings$/);
});
