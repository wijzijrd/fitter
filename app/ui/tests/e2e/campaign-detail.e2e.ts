import { test, expect, focused, overflowsX } from "./api";

test.use({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });

test.beforeEach(async ({ page }) => {
  await page.goto("/campaigns/c1");
  await expect(page.locator(".bib-controls")).toBeVisible();
});

test("group, period and view pickers share one line", async ({ page }) => {
  const tops = await page.locator(".bib-controls select").evaluateAll((els) => els.map((e) => Math.round(e.getBoundingClientRect().top)));
  expect(tops).toHaveLength(3);
  expect(new Set(tops).size).toBe(1);
  expect(await overflowsX(page)).toBe(false);
});

test("changing the view keeps focus on its picker", async ({ page }) => {
  await page.locator("#bib-view").focus();
  await page.selectOption("#bib-view", "person");
  await expect(page.locator("#bib-view")).toHaveValue("person");
  expect((await focused(page)).id).toBe("bib-view");
});

test("the group picker switches the bib", async ({ page }) => {
  await page.selectOption("#bib-group", "strength");
  await expect(page.locator(".bibs")).toContainText(/strength/i);
});

test("the period picker re-scores the board", async ({ page }) => {
  const req = page.waitForRequest((r) => r.url().endsWith("GetCampaignLeaderboard") && r.postData()!.includes("WEEK"));
  await page.selectOption("#bib-window", { label: "Week" });
  await req;
});

test("the timeline chart draws", async ({ page }) => {
  await page.getByRole("tab", { name: "Timeline" }).click();
  await expect(page.locator(".chart canvas").first()).toBeVisible();
  await expect(page.locator(".chart table").first()).toContainText("Jo");
});
