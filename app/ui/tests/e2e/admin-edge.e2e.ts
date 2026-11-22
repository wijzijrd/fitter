import type { Page } from "@playwright/test";
import { test, expect, focused, overflowsX } from "./api";

test.use({ viewport: { width: 375, height: 900 }, hasTouch: true, isMobile: true });

test.beforeEach(async ({ page }) => {
  await page.goto("/admin");
  await expect(page.getByText("Sam Okafor")).toBeVisible();
});

// Match the name cell exactly: "Jo" would otherwise also match "Joined 18 Nov" in another row.
const row = (page: Page, name: string) => page.locator(".rows li").filter({ has: page.locator(".name", { hasText: new RegExp(`^${name}$`) }) });
const connectError = (page: Page, method: string, status: number, code: string) =>
  page.route(`**/*.AdminService/${method}`, (r) => r.fulfill({ status, contentType: "application/json", body: JSON.stringify({ code, message: "developer-facing detail" }) }));

// --- Users -----------------------------------------------------------------------------

test("approving moves a pending member to Active", async ({ page }) => {
  const req = page.waitForRequest((r) => r.url().endsWith("ApproveUser"));
  await row(page, "Sam Okafor").getByRole("button", { name: "Approve" }).click();
  expect((await req).postDataJSON()).toEqual({ id: "u2" });
  await expect(page.getByRole("heading", { name: "Pending (0)" })).toBeVisible();
  await expect(page.getByText("Nobody waiting.")).toBeVisible();
  await expect(row(page, "Sam Okafor")).toContainText("member");
});

test("your own row has no actions, and other admins can't be banned", async ({ page }) => {
  await expect(row(page, "Alexandria Vandenberg-Smith")).toContainText("you");
  await expect(row(page, "Alexandria Vandenberg-Smith").getByRole("button")).toHaveCount(0);
  await expect(row(page, "Priya").getByRole("button", { name: "Demote" })).toBeVisible();
  await expect(row(page, "Priya").getByRole("button", { name: /Ban/ })).toHaveCount(0);
  await expect(row(page, "Jo").getByRole("button", { name: /Ban/ })).toBeVisible();
});

test("promote and demote swap the role and the button", async ({ page }) => {
  const sent: unknown[] = [];
  page.on("request", (r) => r.url().endsWith("SetUserRole") && sent.push(r.postDataJSON()));
  await row(page, "Jo").getByRole("button", { name: "Promote" }).click();
  await expect(row(page, "Jo")).toContainText("admin");
  await expect(row(page, "Jo").getByRole("button", { name: "Demote" })).toBeVisible();
  await row(page, "Priya").getByRole("button", { name: "Demote" }).click();
  await expect(row(page, "Priya")).toContainText("member");
  expect(sent).toEqual([{ id: "u3", role: "admin" }, { id: "u4", role: "member" }]);
});

test("banning needs a real reason, sends it trimmed, and follows the row to Unban", async ({ page }) => {
  await page.locator('[data-ban="u3"]').click();
  const reason = page.locator("#ban-u3");
  const confirm = page.getByRole("button", { name: "Confirm ban" });
  await expect(confirm).toHaveAttribute("aria-disabled", "true");
  await reason.fill("   ");
  await expect(confirm).toHaveAttribute("aria-disabled", "true"); // spaces are not a reason
  expect(await reason.getAttribute("maxlength")).toBe("500");
  await reason.fill("x".repeat(600));
  expect((await reason.inputValue()).length).toBe(500);
  await reason.fill("  spamming the feed  ");
  await expect(confirm).not.toHaveAttribute("aria-disabled", "true");
  const req = page.waitForRequest((r) => r.url().endsWith("BanUser"));
  await confirm.click();
  expect((await req).postDataJSON()).toEqual({ id: "u3", reason: "spamming the feed" });
  await expect(page.getByRole("heading", { name: "Banned (1)" })).toBeVisible();
  await expect(row(page, "Jo")).toContainText("spamming the feed");
  expect((await focused(page)).text).toBe("Unban");
  await page.locator('[data-unban="u3"]').click();
  await expect(page.getByRole("heading", { name: "Banned (0)" })).toBeVisible();
  await expect(page.getByText("No bans.")).toBeVisible();
  await expect(row(page, "Jo")).toContainText("member");
});

test("a refused ban explains itself in plain language and keeps the form open", async ({ page }) => {
  await connectError(page, "BanUser", 400, "failed_precondition");
  await page.locator('[data-ban="u3"]').click();
  await page.locator("#ban-u3").fill("again");
  await page.getByRole("button", { name: "Confirm ban" }).click();
  const alert = page.getByRole("alert");
  await expect(alert).toContainText("can't be done right now");
  await expect(alert).not.toContainText("developer-facing detail"); // server wording never reaches the page
  await expect(page.locator("#ban-u3")).toHaveValue("again");
  await expect(page.getByRole("heading", { name: "Banned (0)" })).toBeVisible();
});

test("a user who vanished mid-click gets a clear message", async ({ page }) => {
  await connectError(page, "ApproveUser", 404, "not_found");
  await row(page, "Sam Okafor").getByRole("button", { name: "Approve" }).click();
  await expect(page.getByRole("alert")).toContainText("doesn't exist any more");
});

// --- Adding a sport --------------------------------------------------------------------

async function openCatalog(page: Page): Promise<void> {
  await page.locator("#admin-tab-catalog").click();
  await expect(page.locator(".rows .name", { hasText: "Powerlifting" })).toBeVisible();
}

test("a new sport is sent trimmed, listed, and the form clears", async ({ page }) => {
  await openCatalog(page);
  await page.locator("#sport-key").fill("  hyrox  ");
  await page.locator("#sport-label").fill("  Hyrox  ");
  const req = page.waitForRequest((r) => r.url().endsWith("CreateCustomSport"));
  await page.getByRole("button", { name: /Add sport/i }).click();
  expect((await req).postDataJSON()).toEqual({ key: "hyrox", label: "Hyrox" });
  await expect(page.locator(".rows .name", { hasText: "Hyrox" })).toBeVisible();
  await expect(page.locator("#sport-key")).toHaveValue("");
  await expect(page.locator("#sport-label")).toHaveValue("");
  expect(await overflowsX(page)).toBe(false);
});

test("an empty sport form sends nothing and lands on the first missing field", async ({ page }) => {
  await openCatalog(page);
  let sent = 0;
  page.on("request", (r) => r.url().endsWith("CreateCustomSport") && sent++);
  await page.getByRole("button", { name: /Add sport/i }).click();
  expect((await focused(page)).id).toBe("sport-key");
  await page.locator("#sport-key").fill("hyrox");
  await page.getByRole("button", { name: /Add sport/i }).click();
  expect((await focused(page)).id).toBe("sport-label");
  expect(sent).toBe(0);
});

test("a refused sport keeps what was typed so it can be corrected", async ({ page }) => {
  await openCatalog(page);
  await connectError(page, "CreateCustomSport", 400, "invalid_argument");
  await page.locator("#sport-key").fill("run");
  await page.locator("#sport-label").fill("Running club");
  await page.getByRole("button", { name: /Add sport/i }).click();
  await expect(page.getByRole("alert")).toContainText("aren't valid");
  await expect(page.locator("#sport-key")).toHaveValue("run");
  await expect(page.locator("#sport-label")).toHaveValue("Running club");
  await expect(page.locator(".rows .name", { hasText: "Running club" })).toHaveCount(0);
});

test("deleting a sport a metric depends on explains why nothing happened", async ({ page }) => {
  await openCatalog(page);
  await connectError(page, "DeleteCustomSport", 400, "failed_precondition");
  await page.locator("[data-delete-sport]").first().click();
  await page.locator(".confirm .pill--primary").click();
  await expect(page.getByRole("alert")).toContainText("something still depends on it");
  await expect(page.locator(".rows .name", { hasText: "Powerlifting" })).toBeVisible();
});

// --- Adding a metric -------------------------------------------------------------------

test("a new metric splits and trims its sport list, and unit and scopes have sensible defaults", async ({ page }) => {
  await openCatalog(page);
  await expect(page.getByLabel("Single scope")).toBeChecked();
  await expect(page.getByLabel("Aggregate scope")).toBeChecked();
  await expect(page.getByLabel("Nullable")).not.toBeChecked();
  await page.locator("#metric-key").fill(" sled_kg ");
  await page.locator("#metric-label").fill(" Sled ");
  await page.locator("#metric-refs").fill(" strength , hyrox ,, ");
  let req = page.waitForRequest((r) => r.url().endsWith("CreateCustomMetric"));
  await page.getByRole("button", { name: /Add metric/i }).click();
  let body = (await req).postDataJSON();
  expect(body).toMatchObject({ key: "sled_kg", label: "Sled", aggregation: "sum", supportsSingleScope: true, supportsAggregateScope: true });
  expect(body.applicableSportRefs).toEqual(["strength", "hyrox"]);
  await expect(page.locator(".rows .name", { hasText: "Sled" })).toBeVisible();
  await expect(page.locator("#metric-key")).toHaveValue("");

  // Blank means "applies to every sport": an empty list, not [""].
  await page.locator("#metric-key").fill("plain");
  await page.locator("#metric-label").fill("Plain");
  req = page.waitForRequest((r) => r.url().endsWith("CreateCustomMetric"));
  await page.getByRole("button", { name: /Add metric/i }).click();
  body = (await req).postDataJSON();
  expect(body.applicableSportRefs ?? []).toEqual([]);
});

test("deleting a metric asks first and keeps focus in the list", async ({ page }) => {
  await openCatalog(page);
  await page.locator("[data-delete-metric]").first().click();
  expect((await focused(page)).text).toBe("Keep");
  await page.keyboard.press("Enter");
  await expect(page.locator(".rows .name", { hasText: "Reps" })).toBeVisible();
  await page.locator("[data-delete-metric]").first().click();
  await page.locator(".confirm .pill--primary").click();
  await expect(page.locator(".rows .name", { hasText: "Reps" })).toBeHidden();
});

// --- Audit and layout ------------------------------------------------------------------

test("the audit tab lists what admins did, and says so when there is nothing", async ({ page }) => {
  await page.locator("#admin-tab-audit").click();
  await expect(page.locator("tr", { hasText: "approve" })).toContainText("Jo");
  await page.unroute("**/*.AdminService/ListAdminActions").catch(() => {});
  await page.route("**/*.AdminService/ListAdminActions", (r) => r.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ actions: [] }) }));
  await page.locator("#admin-tab-users").click();
  await page.locator("#admin-tab-audit").click();
  await expect(page.getByText("Nothing recorded yet.")).toBeVisible();
});

test("every admin tab fits a 320px phone", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 760 });
  for (const tab of ["users", "catalog", "exercises", "audit"]) {
    await page.locator(`#admin-tab-${tab}`).click();
    await expect(page.locator("#admin-panel")).toBeVisible();
    expect(await overflowsX(page), `tab ${tab} overflows horizontally`).toBe(false);
  }
});
