// The what's-new splash. RELEASE is the release's date (YYYY-MM-DD) and names this list: change
// it (and the items) when a release has something to tell members, and everyone sees the splash
// once more. The server only keeps the last RELEASE each member dismissed (users.whats_new_seen).
export const RELEASE = "2026-11-22";

// After a week the news is stale: it is no longer offered on its own, only from Settings.
export const FRESH_DAYS = 7;

export function isFresh(now: number = Date.now(), release: string = RELEASE): boolean {
  return now - Date.parse(release) < FRESH_DAYS * 86_400_000;
}

export interface WhatsNewItem {
  title: string;
  body: string;
}

// Newest first. Plain sentences, no jargon, no PR numbers.
export const whatsNewItems: WhatsNewItem[] = [
  {
    title: "Log gym and circuit sessions properly",
    body: "Pick exercises from a library, group them into blocks, and repeat a block for as many rounds as you did. Supersets, circuits and Hyrox-style sessions all fit. Missing an exercise? Add it from the picker and use it straight away.",
  },
  {
    title: "A new group: HIIT & circuits",
    body: "HIIT, Crossfit and Workout moved out of Strength into their own group. A goal scoped to Strength no longer counts them, so check yours if you log those.",
  },
  {
    title: "The feed shows the last week",
    body: "Activity is grouped by day, your own rows are marked, and a line says what happened since you last looked.",
  },
  {
    title: "Choose what the feed shows",
    body: "Pace or speed, power, effort and the exercises of a session can each be switched on or off. Open Settings from your name at the top.",
  },
  {
    title: "Strength goals and kg targets",
    body: "Log a manual session with weights and set a goal on the heaviest weight you lift for an exercise.",
  },
  {
    title: "Leaderboards by week, month or campaign",
    body: "The leaderboard ranks people by how much of their goals they have finished, over the time frame you pick.",
  },
  {
    title: "Easier goals and quicker typing",
    body: "Create every kind of goal in one place, repeat goals, and use stretch presets. Times are typed as digits and the colons fill in. Outlines are softer, and phones get a menu button.",
  },
  {
    title: "New activities show up on their own",
    body: "Your latest Strava activities now appear when you open the app and while it stays open, with no need to sign out and back in. A brief connection hiccup no longer logs you out either, and when Strava is at capacity you get a clear message.",
  },
];
