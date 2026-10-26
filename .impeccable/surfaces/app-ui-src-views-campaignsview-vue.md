---
version: 1
slug: "app-ui-src-views-campaignsview-vue"
primary_target: "app/ui/src/views/CampaignsView.vue"
related_targets: ["app/ui/src/views/CampaignCreateView.vue","app/ui/src/views/CampaignJoinView.vue"]
---

# Campaigns list, create, join

Mode: Operate. Audience: friend group (2–12) on a phone after a workout or on desktop. Jobs: see where I stand across my campaigns in seconds; start or join a challenge in under a minute on either device.

## Decisions (user-confirmed)
- Built on today's proto (mode, 4 metrics, per-member target); restructure branch (goals, clauses, catalog) slots in later: create gains a "Your goal" step, join sheet becomes the goal builder.
- List: start-list sheets chosen on the decision page (seed:962918d5ab2ce3bc): Racing sheet (live, soonest-ending first), On the start line (upcoming joined, then open to join), Results (finished, FINISHER tag).
- Standing via one GetCampaignLeaderboard call per joined campaign; per-card skeleton and per-card retry; no proto change.
- Pooled campaign card: group % as numeral, your share on the tab.
- Create: /campaigns/new, three steps (name, window, rules), live bib preview, lands on detail.
- Join: pooled joins in one tap; per-member opens /campaigns/:id/join to set own target (prefilled).
- Leave moves from the list to the detail page.

## Direction contract
THESIS: The campaigns list is your race-day start sheet: one row per race with your number, the race, and the next gap in fixed columns, and a tear-off footer naming your closest move. Refuses the white-card list of names with Join/Leave buttons, and the stack of big cards.
OWN-WORLD: Race Bib world from DESIGN.md unchanged: white ground, Black Cherry ink and 2px outlines, Tangerine sheet band for Racing, Cool Blue bands for On the start line and Results, dashed Cool Blue tear-off footer, Bungee numbers and column labels, Hanken Grotesk race names, tabular figures. Flat; no offsets in the list.
STORY: I open Fitter and read down one sheet: 3rd in October Climb 470 m to Sam, 1st in Lunch Run Club 8.2 km clear, the group at 64%; the footer tells me my closest move. Below, what's starting and what I can join; I start my own in three steps while its bib takes shape.
FIRST VIEWPORT: Phone (390). Header bar. "Campaigns" Bungee headline with a Tangerine "New campaign" pill right. Racing sheet: 2px outlined, Tangerine band "START LIST · RACING" with race count, a column row YOUR NO. / RACE / NEXT GAP, rows with a 40px Bungee number, race name and "of 6 · 14 days left", gap right-aligned; dashed Cool Blue footer "Closest move: 470 m to catch Sam in October Climb". On the start line sheet begins in view. Desktop: Racing sheet left (max 640px), On the start line and Results sheets in a side column (max 400px).
FORM: Start list, number 4 of 7 on the re-roll's grounded list, the dealt lead; chosen by the user on the decision page over Bib wall, Move-up list and a steered Race calendar. seed:962918d5ab2ce3bc. Signature interaction: the tear-off footer computes your closest move across every race (smallest gap relative to target); create's live preview bib fills in as you type, instant, no swap.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
