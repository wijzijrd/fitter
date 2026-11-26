// Release notes. Every user-facing change ships as a release with a semver bump (CLAUDE.md,
// "Versions"): major for app-wide updates, minor for new features, patch for fixes and polish.
// The newest release is the app version (package.json must match; a test checks). On app open
// a member sees the releases they haven't, but only when a major or minor is among them.
// The server keeps the last version each member closed (users.whats_new_seen).

export type ReleaseKind = "major" | "minor" | "patch";

export interface ReleaseNote {
  title: string;
  body: string;
}

export interface Release {
  version: string;
  date: string; // YYYY-MM-DD
  notes: ReleaseNote[];
}

export const kindLabels: Record<ReleaseKind, string> = {
  major: "Major update",
  minor: "New features",
  patch: "Fixes and polish",
};

// Newest first. Plain sentences, no jargon, no PR numbers.
export const releases: Release[] = [
  {
    version: "1.0.0",
    date: "2026-11-26",
    notes: [
      {
        title: "Welcome to Kombeweeg",
        body: "Kombeweeg has its own home now at kombeweeg.com. Your account, lifts, custom exercises and feed settings come with you: sign in with Strava and you'll find them where you left them. Your Strava activities are fetched again after you sign in.",
      },
      {
        title: "Goals start fresh",
        body: "Goals didn't move over, so create the ones you want again. Campaigns were recreated by the owner, not copied.",
      },
    ],
  },
  {
    version: "0.8.2",
    date: "2026-11-25",
    notes: [
      {
        title: "Fitter is now Kombeweeg",
        body: "Same app, new name and icon, ahead of a bigger redesign. If you added it to your home screen, add it again to pick up the new icon.",
      },
    ],
  },
  {
    version: "0.8.1",
    date: "2026-11-25",
    notes: [
      {
        title: "Developer feedback is paused",
        body: "It's switched off while email sending is set up properly. It will be back in Settings soon.",
      },
    ],
  },
  {
    version: "0.8.0",
    date: "2026-11-25",
    notes: [
      {
        title: "Release notes",
        body: "Every version and what changed in it is listed in Settings under Release notes. This window now only opens for new features and bigger updates; small fixes are listed there quietly.",
      },
    ],
  },
  {
    version: "0.7.1",
    date: "2026-11-25",
    notes: [
      { title: "This window opens at the top", body: "On short screens the list starts at the first item, and Got it always stays in view." },
      { title: "A clearer feedback page", body: "It says why your email is needed before asking for it." },
    ],
  },
  {
    version: "0.7.0",
    date: "2026-11-25",
    notes: [
      { title: "Settings has its own menu", body: "Open it from your name at the top. Feed details now live under App permissions." },
      { title: "Developer feedback", body: "Send the developer a bug report or an idea from Settings. You verify your email first, so a reply can reach you." },
    ],
  },
  {
    version: "0.6.0",
    date: "2026-11-22",
    notes: [
      {
        title: "Fairer leaderboards",
        body: "Weekly and daily habits are now scored by the weeks or days you hit, not as one running total, and habits, single-effort goals and totals are ranked in their own groups, so a 66% habit is no longer shown ahead of a 9% distance goal. Your rank may move because of this. A neighbour's gap now says who it is measured against, like \"−57% vs you\".",
      },
      {
        title: "New activities show up on their own",
        body: "Your latest Strava activities now appear when you open the app and while it stays open, with no need to sign out and back in.",
      },
      { title: "What's new after each release", body: "A short list of what changed shows once after a release." },
    ],
  },
  {
    version: "0.5.0",
    date: "2026-11-22",
    notes: [
      { title: "The feed shows the last week", body: "Activity is grouped by day, your own rows are marked, and a line says what happened since you last looked." },
      {
        title: "Choose what the feed shows",
        body: "Pace or speed, power, effort and the exercises of a session can each be switched on or off in Settings.",
      },
    ],
  },
  {
    version: "0.4.0",
    date: "2026-11-22",
    notes: [
      {
        title: "Log gym and circuit sessions properly",
        body: "Pick exercises from a library, group them into blocks, and repeat a block for as many rounds as you did. Supersets, circuits and Hyrox-style sessions all fit. Missing an exercise? Add it from the picker and use it straight away, and set a goal on the heaviest weight you lift for any exercise.",
      },
      {
        title: "A new group: HIIT & circuits",
        body: "HIIT, Crossfit and Workout moved out of Strength into their own group. A goal scoped to Strength no longer counts them, so check yours if you log those.",
      },
      { title: "Softer outlines", body: "Cards and buttons have lighter borders." },
    ],
  },
  {
    version: "0.3.1",
    date: "2026-11-21",
    notes: [
      {
        title: "Quicker typing, easier tapping",
        body: "Times are typed as digits and the colons fill in, phones get a menu button, and buttons are big enough to tap comfortably.",
      },
    ],
  },
  {
    version: "0.3.0",
    date: "2026-11-21",
    notes: [
      { title: "Easier goals", body: "Create every kind of goal in one place, repeat goals every day, week or month, and use stretch presets." },
      { title: "Log activities by hand", body: "Add an activity Strava doesn't have, like a gym session, and set strength goals in kilograms." },
    ],
  },
  {
    version: "0.2.0",
    date: "2026-11-21",
    notes: [
      {
        title: "Leaderboards by week, month or campaign",
        body: "The leaderboard ranks people by how much of their goals they have finished, over the time frame you pick.",
      },
      {
        title: "Steadier sign-in",
        body: "A brief connection hiccup no longer logs you out, and when Strava is at capacity you get a clear message.",
      },
    ],
  },
  {
    version: "0.1.0",
    date: "2026-11-17",
    notes: [{ title: "First beta", body: "Sign in with Strava, join a campaign, set goals and see how everyone is doing." }],
  },
];

export const CURRENT = releases[0].version;

// The splash before versions existed was named by its date; it shipped with 0.6.0.
const LEGACY: Record<string, string> = { "2026-11-22": "0.6.0" };

export function parseVersion(v: string): [number, number, number] | null {
  const m = /^(\d+)\.(\d+)\.(\d+)$/.exec(LEGACY[v] ?? v);
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null;
}

export function compareVersions(a: string, b: string): number {
  const x = parseVersion(a) ?? [0, 0, 0];
  const y = parseVersion(b) ?? [0, 0, 0];
  return x[0] - y[0] || x[1] - y[1] || x[2] - y[2];
}

export function kindOf(version: string): ReleaseKind {
  const [major, minor, patch] = parseVersion(version) ?? [0, 0, 0];
  if (patch > 0) return "patch";
  return major > 0 && minor === 0 ? "major" : "minor";
}

function newestFeature(list: Release[]): Release | undefined {
  return list.find((r) => kindOf(r.version) !== "patch");
}

// True when a major or minor release is newer than the one the member last closed.
export function shouldPopUp(seen: string, list: Release[] = releases): boolean {
  const feature = newestFeature(list);
  if (!feature) return false;
  return !parseVersion(seen) || compareVersions(feature.version, seen) > 0;
}

// The releases newer than the one last closed, newest first. Someone who never closed one sees
// the newest feature release and anything after it, not the whole history.
export function unseenReleases(seen: string, list: Release[] = releases): Release[] {
  if (parseVersion(seen)) return list.filter((r) => compareVersions(r.version, seen) > 0);
  const feature = newestFeature(list);
  return feature ? list.slice(0, list.indexOf(feature) + 1) : list.slice(0, 1);
}
