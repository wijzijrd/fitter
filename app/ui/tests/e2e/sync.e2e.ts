import { test, expect } from "./api";

test.use({ viewport: { width: 390, height: 900 }, hasTouch: true, isMobile: true });

test("signing in asks Strava for new activities once, and nothing reloads when there are none", async ({ page }) => {
  const syncs: string[] = [];
  const feeds: string[] = [];
  page.on("request", (r) => {
    if (r.url().endsWith("SyncActivities")) syncs.push(r.url());
    if (r.url().endsWith("GetCampaignFeed")) feeds.push(r.url());
  });
  await page.goto("/campaigns/c1");
  await expect(page.getByRole("tab", { name: "Feed" })).toBeVisible();
  await expect.poll(() => syncs.length).toBe(1);
  await page.waitForTimeout(300);
  expect(feeds.length).toBe(1);
});

test("when the sync finds new activities, the open campaign reloads", async ({ page }) => {
  // Strava takes seconds, so the page is already showing by the time the answer lands.
  await page.route("**/*.ActivityService/SyncActivities", async (route) => {
    await new Promise((r) => setTimeout(r, 400));
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ imported: 2 }) });
  });
  const feeds: string[] = [];
  page.on("request", (r) => r.url().endsWith("GetCampaignFeed") && feeds.push(r.url()));
  await page.goto("/campaigns/c1");
  await expect(page.getByRole("tab", { name: "Feed" })).toBeVisible();
  await expect.poll(() => feeds.length).toBe(2);
});

test("coming back to the tab within five minutes doesn't ask again", async ({ page }) => {
  const syncs: string[] = [];
  page.on("request", (r) => r.url().endsWith("SyncActivities") && syncs.push(r.url()));
  await page.goto("/");
  await expect.poll(() => syncs.length).toBe(1);
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(300);
  expect(syncs.length).toBe(1);
});
