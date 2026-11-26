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

test("sign-ups: invite-only takes an athlete id and removes it again", async ({ page }) => {
  await expect(page.getByRole("heading", { name: "Sign-ups" })).toBeVisible();
  await page.locator("#reg-mode").selectOption("invite_only");
  await expect(page.getByText("Nobody invited.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Invite athlete" })).toBeDisabled();
  await page.locator("#allow-id").fill("424242");
  await page.getByRole("button", { name: "Invite athlete" }).click();
  await expect(page.getByText("Strava athlete 424242")).toBeVisible();
  await page.getByRole("button", { name: "Remove athlete 424242" }).click();
  await expect(page.getByText("Strava athlete 424242")).toBeHidden();
});

test("ban form takes focus and gives it back on Cancel", async ({ page }) => {
  await page.locator('[data-ban="u2"]').click();
  expect((await focused(page)).id).toBe("ban-u2");
  await page.getByRole("button", { name: "Cancel" }).click();
  expect((await focused(page)).text).toBe("Ban…");
});

test("exercises: used ones merge, unused ones delete after asking", async ({ page }) => {
  await page.locator("#admin-tab-exercises").click();
  await expect(page.locator(".rows .name", { hasText: "Bench press" })).toBeVisible();
  expect(await overflowsX(page)).toBe(false);

  const bench = page.locator(".rows li", { hasText: "Bench press" });
  await expect(bench.locator("[data-merge]")).toBeVisible();
  await expect(bench.locator("[data-archive]")).toBeVisible(); // used: archive or merge, never delete
  await expect(bench.locator("[data-delete-exercise]")).toHaveCount(0);

  const plank = page.locator(".rows li", { hasText: "Plank" });
  await plank.locator("[data-delete-exercise]").click();
  expect((await focused(page)).text).toBe("Keep");
  await page.locator(".confirm .pill--primary").click();
  await expect(page.locator(".rows .name", { hasText: "Plank" })).toBeHidden();
  expect((await focused(page)).text).toContain("Delete"); // the next unused row's Delete
});

test("exercises: edit takes focus, locks the measure while used, and Escape gives focus back", async ({ page }) => {
  await page.locator("#admin-tab-exercises").click();
  await page.locator('[data-edit="ex-bench"]').click();
  expect((await focused(page)).id).toBe("ex-name-ex-bench");
  await expect(page.locator("#ex-measure-ex-bench")).toBeDisabled();
  expect(await overflowsX(page)).toBe(false);
  await page.keyboard.press("Escape");
  await expect(page.locator("#ex-name-ex-bench")).toBeHidden();
  expect((await focused(page)).text).toContain("Edit");
});

test("exercises: merge asks before moving anything", async ({ page }) => {
  await page.locator("#admin-tab-exercises").click();
  await page.locator('[data-merge="ex-squat"]').click();
  const into = page.locator("#ex-into-ex-squat");
  await expect(into).toBeFocused();
  // Only exercises measured the same way are offered.
  await expect(into.locator("option", { hasText: "SkiErg" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Merge…" })).toHaveAttribute("aria-disabled", "true");
  await into.selectOption({ label: "Front squat" });
  await page.getByRole("button", { name: "Merge…" }).click();
  await expect(page.getByText("Move its 1 uses")).toBeVisible();
  await page.locator(".panel .pill--primary").click();
  await expect(page.locator(".rows .name", { hasText: /^Squat$/ })).toBeHidden();
  await expect(page.locator(".rows li", { hasText: "Front squat" })).toContainText("used 1×");
});

test("exercises: archiving a used exercise hides it from new logs, and Restore brings it back", async ({ page }) => {
  await page.locator("#admin-tab-exercises").click();
  await page.locator('[data-archive="ex-squat"]').click();
  await expect(page.getByRole("heading", { name: "Archived" })).toBeVisible();
  await expect(page.locator(".rows li", { hasText: "Squat" }).filter({ hasText: "used 1×" }).locator("[data-restore]")).toBeVisible();
  expect((await focused(page)).text).toContain("Restore");
  expect(await overflowsX(page)).toBe(false);

  const offered = async () => {
    await page.goto("/activities/new");
    await page.selectOption("#al-sport", "WeightTraining");
    const c = page.locator('input[role="combobox"]').first();
    await c.click();
    await c.fill("squat");
    return page.getByRole("option", { name: /^Squat/ }).count();
  };
  expect(await offered()).toBe(0);

  await page.goto("/admin");
  await page.locator("#admin-tab-exercises").click();
  await page.locator('[data-restore="ex-squat"]').click();
  await expect(page.getByRole("heading", { name: "Archived" })).toBeHidden();
  expect(await offered()).toBe(1);
});
