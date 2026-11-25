import { test, expect, overflowsX } from "./api";

// Mirrors playwright.config.ts: on unless the run sets VITE_FEEDBACK_ENABLED to something else.
const feedbackOn = (process.env.VITE_FEEDBACK_ENABLED ?? "true") === "true";

test("Settings is a menu of App permissions, Developer feedback and Release notes", async ({ page }) => {
  await page.goto("/settings");
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByRole("link", { name: /App permissions/ })).toHaveAttribute("href", "/settings/permissions");
  await expect(page.getByRole("link", { name: /Release notes/ })).toHaveAttribute("href", "/settings/release-notes");
  await expect(page.getByRole("link", { name: /Developer feedback/ })).toHaveCount(feedbackOn ? 1 : 0);
  await page.getByRole("link", { name: /Release notes/ }).click();
  await expect(page.getByRole("heading", { name: "Release notes", level: 1 })).toBeVisible();
  // The navbar name keeps its active underline on the sub-pages.
  await expect(page.getByRole("link", { name: /^Settings, signed in as/ })).toHaveClass(/router-link-active/);
  await page.getByRole("link", { name: "Settings", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
});

test("feedback needs a verified email, then sends kind and message", async ({ page }) => {
  test.skip(!feedbackOn, "Developer feedback is off in this build");
  await page.goto("/settings/feedback");
  // First the page says why an email is needed, and asks for nothing until the member starts.
  await expect(page.getByRole("heading", { name: "Verify your email to send feedback" })).toBeVisible();
  await expect(page.getByRole("textbox")).toHaveCount(0);
  await page.getByRole("button", { name: "Verify email" }).click();

  const address = page.getByLabel("Your email");
  await expect(address).toBeFocused();
  await expect(page.getByText("Step 1 of 2")).toBeVisible();
  await address.fill("alex@example.com");
  const sent = page.waitForRequest((r) => r.url().endsWith("SendEmailCode"));
  await page.getByRole("button", { name: "Send code" }).click();
  expect((await sent).postDataJSON()).toEqual({ email: "alex@example.com" });

  const code = page.getByLabel("6-digit code");
  await expect(code).toBeFocused();
  await expect(page.getByText("Step 2 of 2")).toBeVisible();
  await code.fill("999999");
  await page.getByRole("button", { name: "Verify" }).click();
  await expect(page.getByRole("alert")).toHaveText("That code doesn't match. Check the email and try again.");
  await code.fill("123456");
  await page.getByRole("button", { name: "Verify" }).click();

  await expect(page.getByText("Replies go to alex@example.com.")).toBeVisible();
  await page.getByRole("radio", { name: "Idea" }).check();
  await page.getByRole("textbox", { name: "Message" }).fill("Show my streak on the feed");
  const submitted = page.waitForRequest((r) => r.url().endsWith("SubmitFeedback"));
  await page.getByRole("button", { name: "Send feedback" }).click();
  expect((await submitted).postDataJSON()).toMatchObject({ kind: "idea", message: "Show my streak on the feed" });
  await expect(page.getByRole("status")).toHaveText("Thanks, it's sent.");
  await expect(page.getByRole("textbox", { name: "Message" })).toHaveValue("");
});

test.describe("phone", () => {
  test.use({ viewport: { width: 320, height: 640 }, hasTouch: true, isMobile: true });

  test("menu rows are 44px tall and nothing overflows", async ({ page }) => {
    await page.goto("/settings");
    for (const row of await page.locator("main nav a").all()) {
      expect((await row.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    }
    expect(await overflowsX(page)).toBe(false);
    if (!feedbackOn) return;
    await page.goto("/settings/feedback");
    await page.getByRole("button", { name: "Verify email" }).click();
    await expect(page.getByLabel("Your email")).toBeVisible();
    expect(await overflowsX(page)).toBe(false);
  });
});

test("with feedback off, the card is hidden and the page redirects", async ({ page }) => {
  test.skip(feedbackOn, "Developer feedback is on in this build");
  await page.goto("/settings");
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.getByRole("link", { name: /Developer feedback/ })).toHaveCount(0);
  await page.goto("/settings/feedback");
  await expect(page).toHaveURL(/\/settings$/);
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
});
