import { test, expect, overflowsX } from "./api";

test("the app is Kombeweeg in the nav, the tab title and its icon set", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".wordmark")).toHaveText("Kombeweeg");
  await expect(page).toHaveTitle("Campaigns · Kombeweeg");
  await expect(page.locator('link[rel="manifest"]')).toHaveAttribute("href", "/icons/staging/manifest.json");
  const manifest = await (await page.request.get("/icons/staging/manifest.json")).json();
  expect(manifest).toMatchObject({ name: "Kombeweeg Staging", short_name: "Kombeweeg" });
  for (const href of ["/icons/staging/favicon-32.png", "/icons/staging/apple-touch-icon.png", "/icons/staging/icon-512.png"]) {
    const res = await page.request.get(href);
    expect(res.status(), href).toBe(200);
    expect(res.headers()["content-type"], href).toContain("image/png");
  }
});

test("login says Kombeweeg and keeps the Strava marks", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 640 });
  await page.addInitScript(() => localStorage.removeItem("fitter_token"));
  await page.goto("/login");
  await expect(page.getByRole("heading", { name: "Kombeweeg", level: 1 })).toBeVisible();
  await expect(page.getByRole("img", { name: "Connect with Strava" })).toBeVisible();
  await expect(page.getByRole("img", { name: "Powered by Strava" })).toBeVisible();
  expect(await overflowsX(page)).toBe(false);
});

test("an invite-only refusal says where to go instead", async ({ page }) => {
  await page.addInitScript(() => localStorage.removeItem("fitter_token"));
  await page.goto("/login?error=invite_only");
  await expect(page.getByText("Kombeweeg is invite-only right now")).toBeVisible();
  await expect(page.getByText("kombeweeg.com instead")).toBeVisible();
});

test.describe("release notes", () => {
  test.use({ seen: "2026-11-22" });

  test("announce the new name", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("dialog", { name: "Release notes" })).toContainText("Fitter is now Kombeweeg");
  });
});
