# Release Notes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace "What's new" with versioned Release notes: a Settings page listing every release, and a pop-up on app open only for major and minor releases (0.8.0). Then switch Developer feedback off by build config (0.8.1).

**Architecture:**
- One typed, newest-first `releases` list in `lib/releases.ts`. It is the app version and the source of truth for both the page and the pop-up.
- Pure functions decide what is unseen and whether to pop up, from the version the server already stores in `whats_new_seen`.
- The feedback toggle is a Vite build-time env flag.

**Tech Stack:** Vue 3 + Pinia + vue-router, Vitest, Playwright. No backend change.

**Spec:** `docs/superpowers/specs/2026-11-25-release-notes-design.md`

## Global Constraints

- Semver kinds:
  - `x.0.0` with `x > 0` → **major**, label "Major update"
  - patch digit `> 0` → **patch**, label "Fixes and polish"
  - anything else → **minor**, label "New features"
- The legacy seen value `"2026-11-22"` reads as `0.6.0`. Any other unparseable or empty value means "never closed".
- Proto field, RPC and column names stay as they are (`whatsNewSeen`, `MarkWhatsNewSeen`, `users.whats_new_seen`).
- Member-facing copy says "Release notes", never "What's new".
- `app/ui/package.json` `version` must equal `releases[0].version`.
- Feedback flag: `import.meta.env.VITE_FEEDBACK_ENABLED === "true"` means on. Anything else means off.
- Branch `release-notes`, based on `feedback-gate-whatsnew-scroll` (PR #51).
  - PR 1 (Tasks 1–2) is version 0.8.0.
  - PR 2 (Task 3), branch `feedback-off` stacked on `release-notes`, is version 0.8.1.
  - Both target `staging`.

## Review Focus

1. **A member who closed the old 22 Nov splash:** the pop-up lists exactly 0.7.0, 0.7.1 and 0.8.0, not the whole history. Unit test in Task 1, e2e in Task 2.
2. **Only a patch is newer than what was seen:** no pop-up, but the page still shows the patch. Unit test with a custom list in Task 1.
3. **The "All release notes" link inside the pop-up:** it must close the dialog (and save the version), not leave a modal over the Release notes page. E2E in Task 2.
4. **The seen value is the current version:** no pop-up after a reload, and no repeat save. E2E in Task 2 (reload step).
5. **Feedback off:** a deep link to `/settings/feedback` lands on `/settings`, and the menu shows no feedback card. E2E in Task 3, run with the flag unset.

---

### Task 1: `lib/releases.ts` (versions, history, pop-up rules)

**Files:**
- Create: `app/ui/src/lib/releases.ts` (replaces `lib/whatsNew.ts`; `git mv` then rewrite)
- Create: `app/ui/src/lib/releases.test.ts` (replaces `lib/whatsNew.test.ts`)
- Modify: `app/ui/package.json` (`"version": "0.8.0"`)

**Interfaces (produces):**
```ts
export type ReleaseKind = "major" | "minor" | "patch";
export interface ReleaseNote { title: string; body: string }
export interface Release { version: string; date: string /* YYYY-MM-DD */; notes: ReleaseNote[] }
export const releases: Release[];            // newest first; history per the spec table, 0.1.0 … 0.8.0
export const CURRENT: string;                 // releases[0].version
export const kindLabels: Record<ReleaseKind, string>;
export function parseVersion(v: string): [number, number, number] | null;
export function compareVersions(a: string, b: string): number;   // <0, 0, >0
export function kindOf(version: string): ReleaseKind;
export function shouldPopUp(seen: string, list?: Release[]): boolean;
export function unseenReleases(seen: string, list?: Release[]): Release[];
```

- **`shouldPopUp`:** true when `seen` is unreadable, or when the newest non-patch release in `list` is newer than `seen`.
- **`unseenReleases`:** returns the releases newer than `seen`. If `seen` is unreadable, it returns the releases from the newest non-patch release onward. Order is newest first.

- [ ] **Step 1: Write the failing tests** in `releases.test.ts`:
  - `parseVersion`:
    - `("0.7.1")` → `[0,7,1]`
    - `("2026-11-22")` → `[0,6,0]`
    - `("")`, `("v1")` and `("1.2")` → `null`
  - `kindOf`:
    - `("1.0.0")` → `"major"`
    - `("0.8.0")` → `"minor"`
    - `("0.1.0")` → `"minor"`
    - `("0.7.1")` → `"patch"`
  - `compareVersions("0.10.0", "0.9.0") > 0`: numeric, not string, comparison.
  - With a custom list `L` = 0.9.1 (patch), 0.9.0, 0.8.1, 0.8.0:
    - `shouldPopUp("0.9.0", L) === false` (patch only is newer)
    - `shouldPopUp("0.8.1", L) === true`
    - `shouldPopUp("", L) === true`
    - `unseenReleases("0.8.1", L)` versions are `["0.9.1","0.9.0"]`
    - `unseenReleases("", L)` versions are `["0.9.1","0.9.0"]`
    - `unseenReleases("garbage", L)` gives the same as `""`
  - On the real list:
    - `unseenReleases("2026-11-22").map(r => r.version)` equals `["0.8.0","0.7.1","0.7.0"]`
    - `shouldPopUp(CURRENT) === false`
  - The real list is strictly descending by `compareVersions`, every version parses, and every release has at least one note.
  - `CURRENT` equals `app/ui/package.json`'s `version`: read it with `readFileSync(new URL("../../package.json", import.meta.url), "utf8")`. If `vue-tsc -b` rejects `node:fs` types, check `tsconfig` for `types`, and add `"node"` there only for the test include.
- [ ] **Step 2: Run** `cd app/ui && npx vitest run src/lib/releases.test.ts`. Expected: FAIL (module not found).
- [ ] **Step 3: Implement** `releases.ts`:
  - Move the eight existing notes into their releases per the spec table.
  - Write new notes for 0.1.0, 0.3.1, 0.7.0 (Settings menu, Developer feedback), 0.7.1 (release notes open at the top with Got it always visible; feedback explains verification first) and 0.8.0 (Release notes page, version-based pop-up).
  - Copy style: plain sentences, no jargon, no PR numbers.
  - Delete `FRESH_DAYS` and `isFresh`.
- [ ] **Step 4: Run** the same command. Expected: PASS.
- [ ] **Step 5: Commit** "Versioned release history in lib/releases.ts (0.8.0)".

### Task 2: Pop-up, Release notes page, Settings wiring (0.8.0)

**Files:**
- Rename and modify: `components/WhatsNewDialog.vue` → `components/ReleaseNotesDialog.vue`
- Rename and modify: `stores/whatsNew.ts` → `stores/releaseNotes.ts` (`useReleaseNotesStore`)
- Create: `views/SettingsReleaseNotesView.vue`
- Modify:
  - `router/index.ts`: add route `/settings/release-notes`, name `settings-release-notes`, title "Release notes"
  - `views/SettingsView.vue`: add a third card, "Release notes". Its hint is "What changed in each version. You're on {CURRENT}.". The permissions hint becomes "Which details show in campaign feeds."
  - `views/SettingsPermissionsView.vue`: remove the What's new section and its store import
  - `views/SettingsFeedbackView.vue`: `release: CURRENT`
  - `App.vue`: use the renamed store and dialog
  - `DESIGN.md`: replace "What's new dialog" with "Release notes dialog", add a "Release notes page" section, and update the Settings menu paragraph
- Rename and modify: `tests/e2e/whats-new.e2e.ts` → `tests/e2e/release-notes.e2e.ts`
- Modify: `tests/e2e/api.ts`: the fixture option `unseenSplash: boolean` becomes `seen: string`, default `CURRENT`, and `whatsNewSeen = seen`
- Modify: `tests/e2e/settings.e2e.ts`: the menu test also expects the "Release notes" link

**Interfaces:**
- Consumes `releases`, `CURRENT`, `kindLabels`, `kindOf`, `shouldPopUp` and `unseenReleases` from Task 1.
- `useReleaseNotesStore()` returns:
  - `{ open: Ref<boolean>; shown: Ref<Release[]>; offer(): void; dismiss(): Promise<void> }`
  - `offer` keeps the once-per-page-load and not-pending rules, and uses `shouldPopUp(u.whatsNewSeen)`.
  - `dismiss` calls `auth.markWhatsNewSeen(CURRENT)` unless the stored value already equals `CURRENT`.
  - `show()` is deleted.
- `ReleaseNotesDialog`:
  - takes prop `releases: Release[]` and emits `close`;
  - its footer has a `router-link` "All release notes" to `/settings/release-notes`, whose click calls `dlg.close()`, plus the existing "Got it" (autofocus);
  - it keeps #51's layout, where only the list scrolls.

- [ ] **Step 1: Write the failing e2e tests** in `release-notes.e2e.ts`. Keep #51's focus-trap, Escape, 320px and short-screen tests, renamed to the dialog "Release notes". Drop the clock pinning. Add:
  - `seen: ""`: dialog "Release notes" visible; for each of `unseenReleases("")`, the heading with its version is shown; Escape posts `MarkWhatsNewSeen` with `release === CURRENT`; after a reload there is no dialog.
  - `seen: "2026-11-22"`: the dialog shows version headings for exactly `unseenReleases("2026-11-22")`, which are 0.8.0, 0.7.1 and 0.7.0.
  - `seen: CURRENT` (default): no dialog on `/`.
  - `seen: ""`: clicking "All release notes" hides the dialog, lands on `/settings/release-notes` with heading "Release notes", and posts `MarkWhatsNewSeen`.
  - Page: `/settings/release-notes` shows "You're on version {CURRENT}." and one heading per release in `releases`. The 0.7.1 section shows "Fixes and polish". At 320px nothing overflows.
- [ ] **Step 2: Run** `npx playwright test release-notes settings`. Expected: FAIL (the dialog is still named "What's new", and the route is missing).
- [ ] **Step 3: Implement** the renames, store, dialog, page, route, Settings changes, and the `App.vue` and `api.ts` updates listed above.
  - The page follows the `.st` shell, with the `.back` link to Settings.
  - Each release is a `section` with an `h2` for the version and a Meta line "{kind label} · {d Mon yyyy}" over its notes. Each note is a bold title over a Meta sentence.
- [ ] **Step 4: Run** `npx playwright test && npx vitest run && npm run build`. Expected: all pass, and `vue-tsc` is clean.
- [ ] **Step 5: Commit** "Release notes page and version-based pop-up (0.8.0)", including the `DESIGN.md` and `CLAUDE.md` Versions rule, the spec and this plan. Only those two files from the otherwise untracked `docs/`. Push `release-notes` and open PR 1 into `staging`, noting that it stacks on #51.

### Task 3: Developer feedback off by build config (0.8.1)

**Files:**
- Modify:
  - `app/ui/src/lib/config.ts`: `export const feedbackEnabled = import.meta.env.VITE_FEEDBACK_ENABLED === "true";`
  - `router/index.ts`: `settings-feedback` gets `beforeEnter: () => feedbackEnabled || { name: "settings" }`
  - `views/SettingsView.vue`: filter out the feedback card when off
  - `lib/releases.ts`: add release 0.8.1 with a note that Developer feedback is paused for now and will be back
  - `package.json`: `"version": "0.8.1"`
  - `.github/workflows/deploy-web.yml`: add `VITE_FEEDBACK_ENABLED: ${{ vars.FEEDBACK_ENABLED }}` beside `VITE_API_BASE_URL`, with a one-line comment saying "true" turns it on
  - `app/ui/.env.example`: add `VITE_FEEDBACK_ENABLED=true` with a comment
  - `playwright.config.ts`: webServer env gets `VITE_FEEDBACK_ENABLED: process.env.VITE_FEEDBACK_ENABLED ?? "true"`
  - `tests/e2e/settings.e2e.ts`: the feedback tests run only when the flag is on; a new test runs only when it is off
  - `DESIGN.md`: one sentence on the toggle

- [ ] **Step 1: Write the failing test** in `settings.e2e.ts`:
  - `const feedbackOn = (process.env.VITE_FEEDBACK_ENABLED ?? "true") === "true";`
  - Existing feedback tests: `test.skip(!feedbackOn)`.
  - New test `"with feedback off, the card is hidden and the page redirects"` with `test.skip(feedbackOn)`. It expects:
    - `/settings` has no "Developer feedback" link;
    - `goto("/settings/feedback")` lands on URL `/settings` with heading "Settings".
  - Unit (Task 1's file): `CURRENT === "0.8.1"` holds through the package.json check, and `kindOf("0.8.1") === "patch"`.
- [ ] **Step 2: Run** `VITE_FEEDBACK_ENABLED= npx playwright test settings`. Expected: the new test FAILS (the card is still shown).
- [ ] **Step 3: Implement** the changes listed in Files.
- [ ] **Step 4: Run both modes:**
  - `VITE_FEEDBACK_ENABLED= npx playwright test settings release-notes` → PASS; the off test runs and the feedback tests are skipped.
  - `npx playwright test && npx vitest run && npm run build` → all PASS; flag on by default in tests.
- [ ] **Step 5: Commit** "Developer feedback off unless FEEDBACK_ENABLED=true (0.8.1)". Push `feedback-off` and open PR 2 into `staging`, stacked on PR 1. Its body says how to turn feedback on: `gh variable set FEEDBACK_ENABLED --env staging --body true`, then redeploy the web app.
