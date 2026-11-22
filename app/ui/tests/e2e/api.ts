import { test as base, type Page } from "@playwright/test";
import { RELEASE } from "../../src/lib/whatsNew";

// In-memory stand-in for the Connect API (JSON over POST /api/fitter.v1.<Service>/<Method>).
// Each test gets fresh data; deletes mutate it so list reloads reflect them.
// Signed-in users have already dismissed the current splash unless a test says otherwise.
function fixtures(unseenSplash: boolean) {
  let sports = [
    { id: "s1", key: "powerlifting", label: "Powerlifting" },
    { id: "s2", key: "climbing", label: "Climbing" },
  ];
  let activities: any[] = [
    {
      id: "-1", sportType: "WeightTraining", name: "Push day", startDate: "2026-11-20T08:00:00Z", movingTimeS: "3600",
      lifts: [{ exercise: "bench press", weightKg: 80, reps: 5 }],
      blocks: [{ rounds: 3, stations: [{ exerciseId: "ex-bench", exerciseName: "Bench press", weightKg: 80, reps: 5 }] }],
    },
    { id: "-2", sportType: "WeightTraining", name: "Pull day", startDate: "2026-11-19T08:00:00Z", movingTimeS: "3000", lifts: [], blocks: [] },
  ];
  let feedSettings: Record<string, boolean> = { showPace: true, showPower: true, showEffort: true, showLifts: true };
  let whatsNewSeen = unseenSplash ? "" : RELEASE;
  const me = () => ({ id: "u1", name: "Alexandria Vandenberg-Smith", role: "admin", status: "active", feedSettings, whatsNewSeen });
  const MEASURE = (m: string) => `EXERCISE_MEASURE_${m}`;
  const MUSCLE = (m: string) => `MUSCLE_${m}`;
  let exercises: any[] = [
    { id: "ex-bench", name: "Bench press", key: "bench press", measure: MEASURE("WEIGHT_REPS"), muscles: [MUSCLE("CHEST"), MUSCLE("TRICEPS"), MUSCLE("SHOULDERS")], useCount: 2 },
    { id: "ex-front", name: "Front squat", key: "front squat", measure: MEASURE("WEIGHT_REPS"), muscles: [MUSCLE("QUADS"), MUSCLE("CORE")], useCount: 0 },
    { id: "ex-squat", name: "Squat", key: "squat", measure: MEASURE("WEIGHT_REPS"), muscles: [MUSCLE("QUADS"), MUSCLE("GLUTES")], useCount: 1 },
    { id: "ex-pullup", name: "Pull-up", key: "pull-up", measure: MEASURE("REPS"), muscles: [MUSCLE("BACK"), MUSCLE("BICEPS")], useCount: 0 },
    { id: "ex-ski", name: "SkiErg", key: "skierg", measure: MEASURE("DISTANCE"), muscles: [MUSCLE("FULL_BODY"), MUSCLE("CARDIO")], useCount: 0 },
    { id: "ex-sled", name: "Sled push", key: "sled push", measure: MEASURE("DISTANCE"), loaded: true, muscles: [MUSCLE("QUADS"), MUSCLE("GLUTES")], useCount: 0 },
    { id: "ex-plank", name: "Plank", key: "plank", measure: MEASURE("TIME"), muscles: [MUSCLE("CORE")], useCount: 0 },
  ];
  const byName = () => exercises.sort((a, b) => a.name.localeCompare(b.name));
  const nameOf = (id: string) => exercises.find((e) => e.id === id)?.name ?? "";
  const handlers: Record<string, (body: any) => unknown> = {
    "UserService/GetCurrentUser": () => ({ user: me() }),
    "UserService/UpdateFeedSettings": (b: any) => {
      feedSettings = { showPace: false, showPower: false, showEffort: false, showLifts: false, ...b.feedSettings };
      return { user: me() };
    },
    "ActivityService/SyncActivities": () => ({ throttled: true }),
    "UserService/MarkWhatsNewSeen": (b: any) => {
      whatsNewSeen = b.release;
      return { user: me() };
    },
    "CatalogService/ListSports": () => ({
      groups: [
        { group: "run", label: "Running", sportTypes: ["Run"] },
        { group: "strength", label: "Strength", sportTypes: ["WeightTraining"] },
        { group: "circuits", label: "HIIT & circuits", sportTypes: ["Crossfit"] },
      ],
    }),
    "CatalogService/ListMetrics": () => ({
      metrics: [{ metric: "ACTIVITY_METRIC_DISTANCE", key: "distance", label: "Distance", unit: "METRIC_UNIT_METERS", aggregation: "METRIC_AGGREGATION_SUM", supportsAggregateScope: true }],
    }),
    "ExerciseService/ListExercises": () => ({ exercises: byName() }),
    "ExerciseService/CreateExercise": (b: any) => {
      const key = String(b.name).trim().toLowerCase().replace(/\s+/g, " ");
      const have = exercises.find((e) => e.key === key);
      if (have) return { exercise: have, alreadyExisted: true };
      const e = { id: `ex-new-${exercises.length}`, name: String(b.name).trim(), key, measure: b.measure, loaded: !!b.loaded, muscles: b.muscles, useCount: 0 };
      exercises.push(e);
      return { exercise: e };
    },
    "ActivityService/CreateManualActivity": (b: any) => {
      const a = {
        ...b.activity, id: "-9", lifts: [],
        blocks: (b.activity.blocks ?? []).map((bl: any) => ({ ...bl, stations: bl.stations.map((st: any) => ({ ...st, exerciseName: nameOf(st.exerciseId) })) })),
      };
      activities = [a, ...activities];
      return { activity: a };
    },
    "ActivityService/ListManualActivities": () => ({ activities }),
    "AdminService/UpdateExercise": (b: any) => {
      const e = exercises.find((x) => x.id === b.id);
      Object.assign(e, { name: b.name, key: String(b.name).trim().toLowerCase(), measure: b.measure, loaded: !!b.loaded, muscles: b.muscles });
      return { exercise: e };
    },
    "AdminService/SetExerciseArchived": (b: any) => {
      const e = exercises.find((x) => x.id === b.id);
      e.archived = !!b.archived;
      return { exercise: e };
    },
    "AdminService/DeleteExercise": (b: any) => {
      exercises = exercises.filter((e) => e.id !== b.id);
      return {};
    },
    "AdminService/MergeExercises": (b: any) => {
      const src = exercises.find((e) => e.id === b.sourceId);
      const dst = exercises.find((e) => e.id === b.targetId);
      dst.useCount += src.useCount;
      exercises = exercises.filter((e) => e.id !== b.sourceId);
      return { exercise: dst };
    },
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
    "CampaignService/ListCampaigns": () => ({
      campaigns: [{ id: "c1", name: "November Base", startDate: "2026-11-17T00:00:00Z", endDate: "2026-12-15T23:59:59Z", joined: true, memberCount: 6, goalCount: 6 }],
    }),
    "GoalService/ListGoals": () => ({ goals: [] }),
    // Times are relative to now: the app shows only the last 7 days. Alexandria last looked 5 hours ago.
    "FeedService/GetCampaignFeed": () => {
      const ago = (h: number) => new Date(Date.now() - h * 3_600_000).toISOString();
      const station = (name: string) => ({ exerciseName: name, weightKg: 60, reps: 8 });
      return {
        seenAt: ago(5),
        activities: [
          { userId: "u3", userName: "Jo Park", name: "Morning run", sportType: "Run", sportGroup: "run", startDate: ago(1), distanceM: 5000, movingTimeS: "1560", stravaActivityId: "101", source: "strava", sufferScore: 42 },
          { userId: "u1", userName: "Alexandria Vandenberg-Smith", name: "Evening ride", sportType: "Ride", sportGroup: "ride", startDate: ago(3), distanceM: 30000, movingTimeS: "3600", stravaActivityId: "102", source: "strava", averageWatts: 212.4 },
          {
            userId: "u4", userName: "Sam Okafor", name: "Leg day", sportType: "WeightTraining", sportGroup: "strength", startDate: ago(4), movingTimeS: "3000", stravaActivityId: "-5", source: "manual",
            blocks: ["Squat", "Leg press", "Lunge", "Calf raise"].map((n) => ({ rounds: 1, stations: [station(n)] })),
          },
          { userId: "u5", userName: "Rita Mendes", name: "Ancient ride", sportType: "Ride", sportGroup: "ride", startDate: ago(24 * 10), distanceM: 40000, movingTimeS: "5400", stravaActivityId: "103", source: "strava" },
        ],
      };
    },
    "FeedService/MarkFeedSeen": () => ({}),
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

async function mockApi(page: Page, unseenSplash: boolean): Promise<void> {
  const handlers = fixtures(unseenSplash);
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
export const test = base.extend<{ unseenSplash: boolean }>({
  unseenSplash: [false, { option: true }],
  page: async ({ page, unseenSplash }, use) => {
    await mockApi(page, unseenSplash);
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
