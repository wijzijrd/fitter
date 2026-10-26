# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

A close group of friends who already log their activities on Strava. They sign in with Strava, join shared campaigns, and keep private personal goals. They switch between phone and desktop, so every flow must work equally well on both: no "use desktop for setup" split.

## Product Purpose

Fitter turns the activities friends already record on Strava into shared challenges and personal goals. Activities arrive automatically through Strava's webhook, so nobody logs anything twice. Success means the group keeps itself accountable: people open Fitter to see where they stand in a campaign, see what friends did, and check their own goals.

## Positioning

Strava Clubs and Challenges can't offer three things Fitter does:

- **Custom challenge rules.** The group sets its own target, metric, and date range, and chooses whether progress is pooled toward one shared target or tracked per member. Custom sports and metrics (e.g. strength work) extend what counts.
- **Private personal goals** that sit alongside the shared campaigns and are never visible to anyone else.
- **Friend-scale accountability.** A feed, leaderboard, and progress timeline for each campaign, built for a small crew rather than a public club.

## Operating Context

- Sign-in is Strava OAuth only; there are no Fitter-specific accounts.
- Activity data flows in passively from Strava webhooks. Users don't enter activities in Fitter.
- Core surfaces: campaigns list, campaign detail (feed, leaderboard, timeline), personal goals, login.
- Users check progress on a phone during the day or after a workout, and set up campaigns and goals on whichever device is at hand.

## Capabilities and Constraints

- **Campaigns:** name, description, metric (distance, elevation, time, count), target value, start/end dates, mode `pooled` or `per_member`. Members join and leave, and in per-member mode a member may override the target for themselves.
- **Goals:** private, per-user target on a metric over a period, with computed progress.
- **In progress** (branch `campaign-goal-restructure`, unmerged): a catalog of metrics and sport groups (units, aggregation, lower-is-better, single vs aggregate scope), admin-managed custom sports and custom metrics, and an admin role gated server-side.
- **Stack:** Vue 3 + Vite + Pinia + ECharts UI, Go ConnectRPC backend, Postgres. The proto definitions in `proto/fitter/v1` are the source of truth for domain terms.
- Strava allows one webhook subscription per API app, so staging and prod need separate Strava apps.

## Brand Commitments

- The name **Fitter** is fixed.
- **Strava brand guidelines are binding:** use the official "Connect with Strava" button for sign-in and show "Powered by Strava" attribution wherever Strava data is displayed, as the Strava API terms require. Don't imply Strava endorsement, and don't restyle Strava's marks.

## Evidence on Hand

- Real content comes only from users' own Strava activities and the campaigns and goals they create.
- There are no testimonials, user counts, press, or marketing assets. Don't fabricate any.

## Product Principles

1. **Strava is the source; Fitter is the lens.** Never ask users to re-enter what Strava already knows.
2. **Private stays private.** Personal goals never leak into shared views, feeds, or leaderboards.
3. **Your crew, your rules.** Campaign rules are flexible enough that the group can define the challenge it actually wants.
4. **Equal on phone and desktop.** Every task completes comfortably on either device.
5. **Standing at a glance.** Where you and your friends stand should be readable in seconds.

## Accessibility & Inclusion

WCAG 2.2 AA is required across all surfaces. Charts (leaderboard, timeline, goal progress) must expose their data in a non-visual form and must not rely on color alone.
