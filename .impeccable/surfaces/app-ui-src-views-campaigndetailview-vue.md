---
version: 1
slug: "app-ui-src-views-campaigndetailview-vue"
primary_target: "app/ui/src/views/CampaignDetailView.vue"
related_targets: []
---

# Campaign detail

Mode: Operate. Audience: a close friend group checking standing on a phone after a workout or mid-day; desktop equally supported. Job: in five seconds, see my rank, the gap to the friend ahead and behind, and feel the competition.

## Decisions (user-confirmed)
- Pooled campaigns: one group bib (group total vs target); its tear-off tab carries your contribution; the peeks are the top two contributors; the list ranks contributions.
- Rank movement (▲/▼): dropped until the API carries rank history.
- Private goal peek: one line under your bib, labelled private, only the viewer's own goal for this campaign's metric that covers today.
- Not joined: a Join control replaces your bib (reuses campaigns store joinCampaign).
- Leaderboard chart removed; stack plus standings list replace it.
- Unchanged: proto/API, routes, Strava sign-in, Standings / Feed / Timeline tabs.

## Direction contract
THESIS: Standing is a race bib you wear, not a table you scan: your rank in monumental numerals with the next gap on the tear-off tab. Refuses the white-card leaderboard-plus-line-chart dashboard.
OWN-WORLD: White ground, Black Cherry #351E28 ink and 2px outlines, Tangerine #FF5C34 bib bands and buttons as fills only, Cool Blue #D7EFFF peek bibs, tear-off tab and FINISHER. Bungee capitals for numerals and titles, Hanken Grotesk for names and reading, tabular figures. Flat; a single hard offset only under the front bib.
STORY: I open the campaign, see I am 3rd and 8.2 km behind Sam, glance at who is behind me, step through the field with Ahead / Behind, and check the feed or timeline.
FIRST VIEWPORT: Phone (390). Header: Fitter wordmark, Campaigns / Goals. Campaign title in Bungee, meta line (mode, metric, Day 18 of 31, members). Tabs. The Bib Stack: friend-ahead bib peeking top-left, friend-behind bib bottom-right (Cool Blue bands carrying the gap), your bib front and centre with a Tangerine band, rank about 104px, progress, and a Cool Blue dashed tear-off tab "8.2 km to catch Sam". Ahead / Behind buttons; standings begin below. Desktop: stack column left, tabs and content right.
FORM: Race Bib world, number 3 on the grounded list, chosen from the safer hand; Bib Stack structure, number 3 of 7 from the surface roll; Bungee type pairing. seed:aa7e9553e219dcac. Signature interaction: stepping through the field brings a neighbour's bib to the front with one orchestrated swap (exponential ease-out; instant under reduced motion), with swipe as a supplement to the buttons.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
