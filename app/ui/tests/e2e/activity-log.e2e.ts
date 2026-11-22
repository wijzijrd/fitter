import type { Page } from "@playwright/test";
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

// The exercise pickers only: the sport <select> is a combobox too.
const combo = (page: Page, n = 0) => page.locator('input[role="combobox"]').nth(n);
// Type to filter, Enter takes the highlighted match.
async function choose(page: Page, n: number, text: string): Promise<void> {
  const c = combo(page, n);
  await c.click();
  await c.fill(text);
  await c.press("Enter");
}

test("history describes a session in one line", async ({ page }) => {
  await expect(page.locator(".history small").first()).toContainText("Bench press 80 kg × 5 · 3 rounds");
});

test("exercise rows stay usable at 320px", async ({ page }) => {
  await page.selectOption("#al-sport", "WeightTraining");
  await choose(page, 0, "bench");
  await expect(combo(page)).toHaveValue("Bench press");
  const kg = page.getByPlaceholder("kg", { exact: true });
  await expect(kg).toBeVisible();
  expect((await kg.boundingBox())!.width).toBeGreaterThan(80);
  // Every control a thumb has to hit is at least 44px tall.
  for (const el of [combo(page), kg, page.getByRole("button", { name: /^Add exercise/ }), page.getByRole("button", { name: "More rounds" }), page.getByRole("button", { name: "Add block" })]) {
    expect((await el.boundingBox())!.height).toBeGreaterThanOrEqual(43.5);
  }
  expect(await overflowsX(page)).toBe(false);

  await page.getByRole("button", { name: /^Add exercise/ }).click();
  expect((await focused(page)).id).toMatch(/-ex-/);
  expect(await overflowsX(page)).toBe(false);
});

test("the exercise picker works from the keyboard", async ({ page }) => {
  await page.selectOption("#al-sport", "WeightTraining");
  const c = combo(page);
  await c.focus();
  await expect(c).toHaveAttribute("aria-expanded", "true");
  await c.fill("pl");
  await expect(page.getByRole("option", { name: /Plank/ })).toBeVisible();
  await c.press("Escape");
  await expect(c).toHaveAttribute("aria-expanded", "false");
  expect((await focused(page)).id).toBe(await c.getAttribute("id"));

  await c.fill("ski");
  await expect(c).toHaveAttribute("aria-expanded", "true");
  await c.press("ArrowDown"); // the "Add ..." option comes after the matches
  await expect(page.getByRole("option", { name: /Add “ski”/ })).toHaveAttribute("aria-selected", "true");
  await c.press("ArrowUp");
  await c.press("Enter");
  await expect(c).toHaveValue("SkiErg");
  await expect(c).toHaveAttribute("aria-expanded", "false");
  await expect(page.getByPlaceholder("metres")).toBeVisible();
});

test("a missing exercise is added from the picker without leaving the form", async ({ page }) => {
  await page.selectOption("#al-sport", "WeightTraining");
  await combo(page).click();
  await combo(page).fill("Zercher squat");
  await combo(page).press("Enter"); // the only option is "Add ..."
  const form = page.getByRole("group", { name: "New exercise" });
  await expect(form).toBeVisible();
  await expect(form.getByLabel("Name")).toHaveValue("Zercher squat");

  await form.getByRole("button", { name: "Add exercise" }).click();
  await expect(form.getByText("Pick how it's measured.")).toBeVisible();
  await expect(form.getByText("Pick at least one muscle.")).toBeVisible();
  await form.getByLabel("Weight × reps").check();
  await form.getByLabel("Quads", { exact: true }).check();
  await form.getByRole("button", { name: "Add exercise" }).click();

  await expect(form).toBeHidden();
  await expect(combo(page)).toHaveValue("Zercher squat");
  expect((await focused(page)).id).toBe(await combo(page).getAttribute("id"));
  await expect(page.getByPlaceholder("kg", { exact: true })).toBeVisible();
});

test("an incomplete row sends focus to the field that needs filling", async ({ page }) => {
  await page.selectOption("#al-sport", "WeightTraining");
  await choose(page, 0, "bench");
  await page.locator('button[type="submit"]').click();
  await expect(page.getByText("Weight and reps are both needed.")).toBeVisible();
  expect((await focused(page)).id).toMatch(/-w-/);
});

test("a superset circuit is sent as blocks with rounds", async ({ page }) => {
  await page.selectOption("#al-sport", "Crossfit");
  await expect(page.locator("#al-distance")).toBeHidden(); // distance belongs to a station here

  await choose(page, 0, "bench");
  await page.getByPlaceholder("kg", { exact: true }).fill("70");
  await page.getByPlaceholder("reps", { exact: true }).fill("8");
  await page.getByRole("button", { name: /^Add exercise/ }).click();
  await choose(page, 1, "pull");
  await page.getByPlaceholder("reps", { exact: true }).nth(1).fill("8");
  await page.getByRole("button", { name: "More rounds" }).click();
  await page.getByRole("button", { name: "More rounds" }).click();
  await expect(page.locator("output")).toHaveText("3 rounds");

  await page.getByRole("button", { name: "Add block" }).click();
  await choose(page, 2, "ski");
  await page.getByPlaceholder("metres").fill("1000");

  const req = page.waitForRequest((r) => r.url().endsWith("CreateManualActivity"));
  await page.locator('button[type="submit"]').click();
  const { activity } = (await req).postDataJSON();
  expect(activity.sportType).toBe("Crossfit");
  expect(activity.blocks).toHaveLength(2);
  expect(activity.blocks[0].rounds).toBe(3);
  expect(activity.blocks[0].stations).toMatchObject([
    { exerciseId: "ex-bench", weightKg: 70, reps: 8 },
    { exerciseId: "ex-pullup", reps: 8 },
  ]);
  expect(activity.blocks[1].stations).toMatchObject([{ exerciseId: "ex-ski", distanceM: 1000 }]);

  await expect(page.locator(".history small").first()).toContainText("3 × (Bench press 70 kg × 8 + Pull-up × 8); SkiErg 1 km");
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
