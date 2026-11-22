import { test as base, type Page } from "@playwright/test";

// In-memory stand-in for the Connect API (JSON over POST /api/fitter.v1.<Service>/<Method>).
// Each test gets fresh data; deletes mutate it so list reloads reflect them.
function fixtures() {
  let sports = [
    { id: "s1", key: "powerlifting", label: "Powerlifting" },
    { id: "s2", key: "climbing", label: "Climbing" },
  ];
  let activities = [
    { id: "-1", sportType: "WeightTraining", name: "Push day", startDate: "2026-11-20T08:00:00Z", movingTimeS: "3600", lifts: [{ exercise: "bench press", weightKg: 80, reps: 5 }] },
    { id: "-2", sportType: "WeightTraining", name: "Pull day", startDate: "2026-11-19T08:00:00Z", movingTimeS: "3000", lifts: [] },
  ];
  return {
    "UserService/GetCurrentUser": () => ({ user: { id: "u1", name: "Alexandria Vandenberg-Smith", role: "admin", status: "active" } }),
    "CatalogService/ListSports": () => ({
      groups: [
        { group: "run", label: "Running", sportTypes: ["Run"] },
        { group: "strength", label: "Strength", sportTypes: ["WeightTraining"] },
      ],
    }),
    "CatalogService/ListMetrics": () => ({ metrics: [] }),
    "ActivityService/ListManualActivities": () => ({ activities }),
    "ActivityService/DeleteManualActivity": (b: { id: string }) => {
      activities = activities.filter((a) => a.id !== b.id);
      return {};
    },
    "AdminService/ListUsers": () => ({
      users: [
        { id: "u2", name: "Sam Okafor", role: "member", status: "pending", createdAt: "2026-11-18T08:00:00Z" },
        { id: "u1", name: "Alexandria Vandenberg-Smith", role: "admin", status: "active" },
        { id: "u3", name: "Jo", role: "member", status: "active" },
      ],
    }),
    "AdminService/ListCustomSports": () => ({ sports }),
    "AdminService/ListCustomMetrics": () => ({ metrics: [{ id: "m1", key: "reps", label: "Reps", unit: "reps", aggregation: "sum" }] }),
    "AdminService/DeleteCustomSport": (b: { id: string }) => {
      sports = sports.filter((s) => s.id !== b.id);
      return {};
    },
    "AdminService/ListAdminActions": () => ({ actions: [] }),
  } as Record<string, (body: any) => unknown>;
}

async function mockApi(page: Page): Promise<void> {
  const handlers = fixtures();
  await page.addInitScript(() => localStorage.setItem("fitter_token", "e2e"));
  await page.route("**/api/fitter.v1.*/**", async (route) => {
    const method = route.request().url().split("/fitter.v1.")[1];
    const handler = handlers[method];
    if (!handler) {
      await route.fulfill({ status: 501, contentType: "application/json", body: JSON.stringify({ code: "unimplemented", message: `no e2e mock for ${method}` }) });
      return;
    }
    const body = handler(JSON.parse(route.request().postData() || "{}"));
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  });
}

// `page` arrives signed in as an admin with the mock API installed.
export const test = base.extend({
  page: async ({ page }, use) => {
    await mockApi(page);
    await use(page);
  },
});
export { expect } from "@playwright/test";

export const focused = (page: Page) =>
  page.evaluate(() => {
    const a = document.activeElement as HTMLElement | null;
    return { tag: a?.tagName ?? "", id: a?.id ?? "", text: a?.textContent?.trim() ?? "" };
  });

export const overflowsX = (page: Page) => page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
