# Release notes (replaces What's new) — design

**Releases:**
- `0.8.0` (minor): Release notes, a new feature.
- `0.8.1` (patch): Developer feedback is turned off by config until SES leaves the sandbox (#52).

## Why

- Staging testers need to see every change, including small fixes.
- Members should only be interrupted for app-wide updates or new features.

Versions follow semver:
- **major:** app-wide update
- **minor:** new feature
- **patch:** bug fix or QOL change

The CLAUDE.md "Versions" rule makes every future change name its bump.

## Approaches

- **A. Beta is 0.x; the production launch is 1.0.0 (chosen).** Past releases are rebuilt from merged PRs, so the page has a real history. The Kombeweeg production launch becomes the first major release, and it pops up for everyone.
- B. Start at 1.0.0 now, with all past notes in one entry. Rejected: no history, and the production launch would have to be 2.0.0.

## Goals

- "What's new" is renamed **Release notes** everywhere members see it.
- **Settings → Release notes** (`/settings/release-notes`) lists every release, newest first:
  - each release shows its version, date, kind and its notes;
  - the top of the page shows the current version.
- **Pop-up on app open:** shown when the newest release's `major.minor` is newer than the last release the member closed.
  - It lists every release they haven't seen, patches included.
  - A member who has never closed one sees only the newest major or minor release and any patches after it.
  - It has "Got it" and a link to all release notes.
  - Patch-only releases never pop up.
- The 7-day limit on the pop-up goes. Someone away for three weeks still sees the new features once.

## Non-goals

- No server, proto or database change.
- No "new" badges on the page.
- No changelog generated from git.

## Constraints and decisions

- **One version number:** `releases[0].version` in the notes list is the app version. A unit test keeps `app/ui/package.json`'s `version` equal to it.
- **Server storage:** the server already stores the last closed release as text (1–32 characters, `users.whats_new_seen`), and the UI now writes the version there.
  - The proto field and column keep their names: renaming them would break old and new containers running side by side during a rollout.
  - The legacy value `2026-11-22` (the What's-new release) is read as `0.6.0`, so existing members only see what's new since then.
  - Any other unreadable value counts as "never closed".
- **Renames in the frontend:**
  - `lib/whatsNew.ts` → `lib/releases.ts`
  - `stores/whatsNew.ts` → `stores/releaseNotes.ts`
  - `WhatsNewDialog` → `ReleaseNotesDialog`
  - `whats-new.e2e.ts` → `release-notes.e2e.ts`
- **App permissions** loses its "What's new" section; the menu hint becomes "Which details show in campaign feeds."
- **Stacking:** this builds on PR #51 (the scroll fix and "Got it" always visible), which touches the same dialog. The branch is based on #51's branch.

## Release history (rebuilt from merged PRs; my call)

| Version | Date | Kind | From |
|---|---|---|---|
| 0.8.0 | 25 Nov 2026 | minor | this change: Release notes |
| 0.7.1 | 25 Nov 2026 | patch | #51: What's new opens at the top; feedback explains verification |
| 0.7.0 | 25 Nov 2026 | minor | #50: Settings menu, Developer feedback |
| 0.6.0 | 22 Nov 2026 | minor | #22: activities sync on their own, fairer leaderboards, What's new |
| 0.5.0 | 22 Nov 2026 | minor | #21: feed shows the last week, choose feed details, Settings |
| 0.4.0 | 22 Nov 2026 | minor | #19, #20: gym and circuit logging, HIIT & circuits group, softer outlines |
| 0.3.1 | 21 Nov 2026 | patch | #15–#18: times typed as digits, phone menu, 44px touch targets |
| 0.3.0 | 21 Nov 2026 | minor | #11–#13: goals in one place, log activities by hand, repeat goals and stretch presets, strength goals |
| 0.2.0 | 21 Nov 2026 | minor | #9, #10, #14: leaderboards by week, month or campaign; a connection blip no longer logs you out |
| 0.1.0 | 17 Nov 2026 | minor | #5–#8: first beta — sign in with Strava, campaigns, goals and a leaderboard |

The existing eight notes are split across these releases. New notes are written for 0.1.0, 0.3.1 and 0.7.0–0.8.0.

## 0.8.1: Developer feedback off by config (the user's decision)

- Developer feedback stays in the 0.7.0 notes. It was released there, on staging.
- **Toggle:** `VITE_FEEDBACK_ENABLED`, baked in at build time. `deploy-web.yml` fills it from a `FEEDBACK_ENABLED` GitHub environment variable.
  - Unset, or anything but `true`, means off. Shipping 0.8.1 therefore turns it off with no infra or variable change.
  - To turn it on per environment: `gh variable set FEEDBACK_ENABLED --env staging --body true`, then redeploy the web app.
- **When off:**
  - The Settings menu hides the Developer feedback card.
  - `/settings/feedback` redirects to `/settings`.
  - The API is untouched: its RPCs still exist, but nothing in the UI calls them.
- The 0.8.1 note says Developer feedback is paused for now and will be back.

## Left to the builder

- Copy, layout and DESIGN.md wording, following the current dialog and Settings pages.
- The kind labels: "Major update", "New features", "Fixes and polish".
