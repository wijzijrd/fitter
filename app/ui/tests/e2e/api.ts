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
  const handlers: Record<string, (body: any) => unknown> = {
    "UserService/GetCurrentUser": () => ({ user: { id: "u1", name: "Alexandria Vandenberg-Smith", role: "admin", status: "active" } }),
    "CatalogService/ListSports": () => ({
      groups: [
        { group: "run", label: "Running", sportTypes: ["Run"] },
        { group: "strength", label: "Strength", sportTypes: ["WeightTraining"] },
      ],
    }),
    "CatalogService/ListMetrics": () => ({
      metrics: [{ metric: "ACTIVITY_METRIC_DISTANCE", key: "distance", label: "Distance", unit: "METRIC_UNIT_METERS", aggregation: "METRIC_AGGREGATION_SUM", supportsAggregateScope: true }],
    }),
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
    ...campaignDetail(),
  };
  return handlers;
}

// Campaign c1: six members with own goals in three sport groups, and a distance timeline.
const DIST = "ACTIVITY_METRIC_DISTANCE";
const people = [
  ["u1", "Alexandria Vandenberg-Smith"], ["u3", "Jo Park"], ["u4", "Sam Okafor"],
  ["u5", "Rita Mendes"], ["u6", "Lee Chan"], ["u7", "Ana Silva"],
] as const;
function entries(group: string, km: number[]) {
  return people.slice(0, km.length).map(([userId, userName], i) => ({
    userId, userName, goalId: `${group}-${userId}`, goalName: `${group} goal`, rank: i + 1,
    progressRatio: km[i] / 100,
    clauses: [{ clauseId: `${group}-${userId}-c`, position: 0, currentValue: km[i] * 1000, hasValue: true, progressRatio: km[i] / 100 }],
    clauseDefs: [{ position: 0, scope: "CLAUSE_SCOPE_AGGREGATE", metric: DIST, comparator: "COMPARATOR_GTE", value: 100000 }],
  }));
}
function campaignDetail() {
  let window = "LEADERBOARD_WINDOW_CAMPAIGN";
  const day = (d: number) => `2026-11-${String(d).padStart(2, "0")}T12:00:00Z`;
  const series = (userId: string, label: string, km: number[]) => ({
    userId, label, goalId: `run-${userId}`, clauseId: `run-${userId}-c`, metric: DIST, comparator: "COMPARATOR_GTE", targetValue: 100000,
    points: km.map((v, i) => ({ date: day(17 + i), value: v * 1000 })),
  });
  return {
    "CampaignService/GetCampaign": () => ({
      campaign: { id: "c1", name: "November Base", startDate: "2026-11-17T00:00:00Z", endDate: "2026-12-15T23:59:59Z", joined: true, memberCount: 6, goalCount: 6 },
    }),
    "CampaignService/ListCampaigns": () => ({ campaigns: [] }),
    "GoalService/ListGoals": () => ({ goals: [] }),
    "FeedService/GetCampaignFeed": () => ({ activities: [] }),
    "FeedService/GetCampaignLeaderboard": (b: { window?: string }) => {
      window = b.window ?? window;
      const scale = window === "LEADERBOARD_WINDOW_WEEK" ? 0.3 : 1;
      return {
        groups: [
          { sportGroup: "run", sportGroupLabel: "Run", entries: entries("run", [62, 48, 40, 31, 22, 9].map((v) => v * scale)) },
          { sportGroup: "strength", sportGroupLabel: "Strength", entries: entries("strength", [70, 55, 20]) },
          { sportGroup: "", sportGroupLabel: "All sports", entries: entries("all", [35, 30]) },
        ],
        people: people.map(([userId, userName], i) => ({ userId, userName, averageRatio: 0.6 - i * 0.08, goalCount: 2, goalsSatisfied: 0, rank: i + 1 })),
      };
    },
    "FeedService/GetCampaignTimeline": () => ({
      groups: [{ sportGroup: "run", sportGroupLabel: "Run", series: [series("u1", "Alexandria Vandenberg-Smith", [5, 18, 30, 44, 62]), series("u3", "Jo Park", [10, 14, 29, 40, 48])] }],
      startDate: "2026-11-17T00:00:00Z", endDate: "2026-12-15T23:59:59Z",
    }),
  };
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
