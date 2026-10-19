---
name: Fitter
description: Strava-fed campaigns and private goals for a close friend group, set as race-day bibs.
colors:
  tangerine: "#ff5c34"
  cool-blue: "#d7efff"
  paper: "#ffffff"
  black-cherry: "#351e28"
  muted-ink: "#5a4a52"
  hairline: "#e9e2e6"
  tangerine-hover: "#ff7a59"
  cool-blue-soft: "#eef8ff"
  cool-blue-deep: "#c3e6ff"
typography:
  display:
    fontFamily: "Bungee, Impact, sans-serif"
    fontSize: "clamp(4.5rem, 24vw, 6.5rem)"
    fontWeight: 400
    lineHeight: 0.85
    fontFeature: "tnum"
  headline:
    fontFamily: "Bungee, Impact, sans-serif"
    fontSize: "clamp(1.625rem, 6.5vw, 2.5rem)"
    fontWeight: 400
    lineHeight: 1.05
  title:
    fontFamily: "Bungee, Impact, sans-serif"
    fontSize: "1.625rem"
    fontWeight: 400
    lineHeight: 1.1
    fontFeature: "tnum"
  label:
    fontFamily: "Bungee, Impact, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.3
  body:
    fontFamily: "Hanken Grotesk Variable, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.5
  body-strong:
    fontFamily: "Hanken Grotesk Variable, system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 700
  meta:
    fontFamily: "Hanken Grotesk Variable, system-ui, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 400
rounded:
  tag: "4px"
  chip: "6px"
  bib: "10px"
  pill: "999px"
spacing:
  gutter: "1rem"
  gutter-desktop: "1.5rem"
  bib-inset: "0.9rem"
  row: "0.7rem"
  stack-gap: "1.25rem"
  pane-gap: "3rem"
components:
  button-primary:
    backgroundColor: "{colors.tangerine}"
    textColor: "{colors.black-cherry}"
    typography: "{typography.body-strong}"
    rounded: "{rounded.pill}"
    padding: "0.5rem 1.1rem"
  button-pill:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.black-cherry}"
    rounded: "{rounded.pill}"
    padding: "0.35rem 0.9rem"
    height: "2.5rem"
  button-pill-hover:
    backgroundColor: "{colors.cool-blue}"
  bib-band:
    backgroundColor: "{colors.tangerine}"
    textColor: "{colors.black-cherry}"
    typography: "{typography.label}"
    padding: "0.3rem 0.75rem"
  bib-band-peek:
    backgroundColor: "{colors.cool-blue}"
    textColor: "{colors.black-cherry}"
    typography: "{typography.label}"
    padding: "0.3rem 0.75rem"
  bib-front:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.black-cherry}"
    rounded: "{rounded.bib}"
  tear-off-tab:
    backgroundColor: "{colors.cool-blue}"
    textColor: "{colors.black-cherry}"
    typography: "{typography.body-strong}"
    padding: "0.5rem 0.9rem"
  rank-chip:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.black-cherry}"
    rounded: "{rounded.chip}"
    height: "2rem"
  rank-chip-you:
    backgroundColor: "{colors.tangerine}"
    textColor: "{colors.black-cherry}"
  finisher-tag:
    backgroundColor: "{colors.cool-blue}"
    textColor: "{colors.black-cherry}"
    rounded: "{rounded.tag}"
    padding: "0.05rem 0.4rem"
  button-pill-primary:
    backgroundColor: "{colors.tangerine}"
    textColor: "{colors.black-cherry}"
    typography: "{typography.body-strong}"
    rounded: "{rounded.pill}"
    padding: "0.35rem 1.1rem"
    height: "2.5rem"
  button-pill-primary-hover:
    backgroundColor: "{colors.tangerine-hover}"
  input:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.black-cherry}"
    typography: "{typography.body}"
    rounded: "{rounded.chip}"
    padding: "0.5rem 0.75rem"
    height: "2.75rem"
  choice-card:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.black-cherry}"
    rounded: "{rounded.bib}"
    padding: "0.7rem 0.9rem"
  choice-card-hover:
    backgroundColor: "{colors.cool-blue-soft}"
  choice-card-checked:
    backgroundColor: "{colors.cool-blue}"
  choice-pill:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.black-cherry}"
    typography: "{typography.body-strong}"
    rounded: "{rounded.pill}"
    padding: "0.45rem 0.9rem 0.45rem 0.7rem"
  choice-pill-checked:
    backgroundColor: "{colors.cool-blue}"
  start-list-row-hover:
    backgroundColor: "{colors.cool-blue-soft}"
---

# Design System: Fitter

## Overview

**Creative North Star: "The Race Bib"**

Fitter looks like race day among friends: vibrant, social, eye-catching, yet legible at a glance. The signature is the bib: a Tangerine band across the top, a monumental Bungee rank numeral, and a dashed tear-off tab carrying the one fact that matters next ("8.2 km to catch Sam"). Standing reads in seconds, on a phone after a workout or on a desktop.

Energy comes from type scale, bold flat colour and 2px Black Cherry outlines, never from texture or ornament. The ground is plain white so bibs, numerals and the two pops carry the screen. Black Cherry does the emphasis work a shadow or a heavier weight would do elsewhere. The theme is light only.

The Bib Stack (your bib in front, friends ahead and behind peeking) is the composition of the campaign detail surface; the Start List (one outlined sheet per race state, a fixed number / race / gap column grid, a tear-off footer) is the composition of the campaigns list. The bib anatomy, palette, type pairing and outline language are the global system. Campaign detail, the campaigns list, and campaign create and join run on it. Pages not yet migrated (login, goals) still run the legacy scaffold tokens and classes in `app/ui/src/style.css` (`--bg`, `--surface`, `--border`, `--text`, `--accent`, `.card`, `.btn`, `.field`, `.progress-bar`); they are pending migration, not part of this system.

**Key Characteristics:**
- White ground, Black Cherry ink and outlines, Tangerine and Cool Blue as flat pops.
- Bib anatomy: band, monumental numeral, dashed tear-off tab; the start-list sheet reuses band and tear-off.
- Bungee capitals for numerals, titles and labels; Hanken Grotesk for names and reading.
- Flat surfaces defined by 2px outlines; one hard offset under the bib in focus.
- Tabular figures everywhere a number appears.

## Colors

A white ground with one deep warm-brown ink and two saturated pops, each with one job.

### Primary
- **Tangerine** (#ff5c34): bib bands, the Racing start-list band, the primary button and primary pill, and the "you" marker: your rank chip, your timeline line (4px), and the 3px underline on the active nav link and active tab. Used as a fill or a heavy indicator stroke, never as text.
- **Tangerine Hover** (#ff7a59): hover fill on Tangerine buttons and primary pills. It lightens rather than darkens so Black Cherry text stays above 4.5:1 (about 6:1).

### Secondary
- **Cool Blue** (#d7efff): peek-bib bands, the "On the start line" and "Results" sheet bands, the tear-off tab and start-list tear-off footer, the FINISHER band and tag, notice panels (including the leave confirmation), checked choice cards and metric pills, hover fill on pills and the logout button, and text selection.
- **Cool Blue Soft** (#eef8ff): hover fill on peek bibs, linked bibs, start-list rows, and unchecked choice cards and metric pills (on hover-capable devices only).
- **Cool Blue Deep** (#c3e6ff): hover fill on the tear-off tab and the linked start-list footer.

### Neutral
- **Paper White** (#ffffff): page ground and bib bodies.
- **Black Cherry** (#351e28): all primary text, rank numerals, 2px outlines, the dashed tear-off rule, focus rings, and the front bib's offset. The "brown for emphasis".
- **Muted Ink** (#5a4a52): secondary text, metadata, start-list column labels, the step count, back links at rest, inactive tabs and nav links, disabled pill text, input placeholders.
- **Hairline** (#e9e2e6): list and start-list row dividers (1px), the tab-bar baseline (2px), disabled pill borders, chart grid lines, the placeholder numeral while a standing loads. Structure only, never an outline on a bib.

### Named Rules
**The Tangerine Is a Fill Rule.** Tangerine never sets text on white (3.1:1 fails AA), and white never sits on Tangerine. Text on Tangerine is Black Cherry (5.0:1). Tangerine may appear as a fill or as a 3px-or-heavier indicator stroke (active underline, your chart line), never as type.

**The No Green Rule.** The palette carries no green in any role, including success and finished states. Finishing is shown in Cool Blue.

**The Strava Orange Is Strava's Rule.** `#fc4c02` appears only inside Strava's official "Connect with Strava" button and "Powered by Strava" mark. Tangerine stays visibly distinct from it.

## Typography

**Display Font:** Bungee 400 (with Impact, sans-serif), self-hosted via @fontsource
**Body Font:** Hanken Grotesk Variable (with system-ui, sans-serif), self-hosted via @fontsource-variable

**Character:** Bungee's upright sign-painter capitals bring loud, sporty legibility to numbers and titles. Hanken Grotesk is a calm grotesque for names and reading.

### Hierarchy
- **Display** (Bungee 400, clamp(4.5rem, 24vw, 6.5rem), 0.85): the rank numeral on the front bib. Drops to clamp(3rem, 15vw, 4rem) for two-digit ranks and percentages, and to 2.75rem on peek bibs. The flat preview bib uses clamp(4rem, 20vw, 5.5rem). Start-list numbers are 2.5rem (line-height 0.9), dropping to 1.5rem past two characters ("64%", "12D").
- **Headline** (Bungee 400, clamp(1.625rem, 6.5vw, 2.5rem), 1.05, uppercase, balanced wrap): campaign titles. The wordmark uses Bungee at 1.25rem.
- **Title** (Bungee 400, 1.625rem, 1.1): the progress value on the front bib (1.125rem when longer than nine characters) "Get your bib" and other in-page Bungee headings such as create's step titles and "Not racing yet" (1.5rem). Bungee 1.625rem also sets the claim pair "No. —", values on preview bibs, and the target input.
- **Label** (Bungee 400, 0.75rem, 1.3, uppercase): bib and sheet band text ("AHEAD", "+8.2 KM", "START LIST · RACING") and start-list column labels (NO. / RACE / NEXT GAP, line-height 1.2, Muted Ink). The FINISHER tag uses 0.6875rem; rank chips use 0.9375rem; create's "n / 3" step count is 0.875rem in Muted Ink.
- **Body** (Hanken Grotesk 400, 1rem; 700 for names, values and controls at 0.875 to 1rem): names, activity lines, buttons, tabs, field labels, start-list race names and gap values, the tear-off tab and footer (700, 0.9375rem). Descriptions cap at 65ch.
- **Meta** (Hanken Grotesk 400, 0.8125 to 0.875rem, Muted Ink): last-active notes, "to go" lines, meta line. Attribution at 0.75rem.

### Named Rules
**The Names Stay Human Rule.** Bungee is capitals only, so people's names, activity titles and anything user-written are set in Hanken Grotesk. Campaign titles are the one user-written string set in Bungee, as headline and bib band text; in start-list rows the race name is Hanken Grotesk 700, clamped to two lines.

**The Tabular Rule.** Every number uses tabular figures (`font-variant-numeric: tabular-nums`) so values align down lists and across bibs.

## Layout

Phone-first, one column with 1rem side gutters inside a 1120px max container. The order is header, tabs, then the bib stack, then the panel; on phones the stack shows only under Standings. From 900px the body becomes two panes: a sticky stack column (minmax(340px, 400px)) on the left and tabs plus panel on the right, 3rem apart, with 1.5rem gutters. Lists are three-column grids (2.25rem rank chip, flexible name, right-aligned value) with 0.7rem row padding and 1px Hairline dividers, so numbers sit in a strict right-hand column. Everything holds at 320px: names and band text truncate with ellipsis rather than wrap.

The campaigns list stacks its sheets in one column, 1.25rem apart. A start-list row is a three-column grid (3.5rem number, flexible race, auto right-aligned gap), 0.75rem gap and inline padding, 0.7rem row padding over 1px Hairline dividers; the column-label row sits on a 2px Black Cherry rule. From 900px the Racing sheet takes a left column (max 640px) and the other sheets a side column (280 to 400px), 2.5rem apart. Create and join pair a preview bib (340 to 400px, sticky on create) with the form (max 560px on create, 480px on join), 3rem apart from 900px; below 900px the preview sits above the form. On phones create's Back / count / Next bar is sticky to the viewport bottom with a 2px Black Cherry top rule and safe-area padding; from 900px it sits inline under the form.

## Elevation & Depth

Surfaces are flat. Bibs, chips, pills and notices are defined by 2px Black Cherry outlines, not shadows. The single depth cue is a hard, unblurred Black Cherry offset (`box-shadow: 0 6px 0 var(--ink)`) under the front bib so it reads as on top of the stack.

**The One Lift Rule.** Only the bib in focus gets the hard offset. Nothing else casts a shadow: preview bibs, empty-state bibs and start-list sheets sit flat. The 1px inset that thickens an invalid input's outline is a stroke, not a shadow.

Motion: bringing a neighbour's bib to the front is one orchestrated swap, entering from the peek's side (translate and scale from 0.9, 380ms on `--ease-out-expo`, cubic-bezier(0.16, 1, 0.3, 1)) while the old bib fades in 140ms. Under reduced motion the swap is instant. Hover fills (pills, peeks, linked bibs, start-list rows, choice cards) transition in 160ms on the same ease. Create's preview bib updates instantly as you type, with no swap.

## Shapes

Bibs, start-list sheets, notices, choice cards and the private-goal box have softly rounded corners (10px) with a 2px outline and clipped bands. Rank chips and text inputs are small rounded squares (6px); the FINISHER tag is tighter (4px). Every button is a pill (999px) with a 2px Black Cherry border. The dashed 2px Black Cherry rule is the tear-off motif: it separates the tear-off tab from the bib and the tear-off footer from a start-list sheet, outlines the private-goal box, and marks quiet pills ("Back to you", "Leave campaign", create's "Cancel"). Tabs are underline tabs, not pills. Metric choices are pills (999px). Icons are 1 to 1.1rem stroked SVG arrows and a padlock, 2px round strokes in currentColor.

## Components

### Buttons
Tactile, outlined, pill-shaped.
- **Shape:** full pill (999px), 2px Black Cherry border.
- **Primary:** Tangerine fill, Black Cherry text, Hanken Grotesk 700 0.9375rem, 0.5rem 1.1rem. Used for "Join campaign" on the unclaimed bib in the stack. Disabled at 60% opacity.
- **Primary pill:** the shared form of the primary action elsewhere: pill metrics (min-height 2.5rem, 0.35rem 1.1rem) with a Tangerine fill, 700 0.9375rem, Tangerine Hover on hover; disabled keeps the Black Cherry border and text at 60% opacity. "New campaign", "Next", "Create campaign", "Join campaign" on the join page, and the confirming "Leave". One per view region.
- **Pill (secondary):** Paper fill, Black Cherry text, 700 0.875rem, min-height 2.5rem. Hover fills Cool Blue. Disabled drops the border to Hairline and text to Muted Ink. The quiet variant uses a dashed border.
- **Focus:** 3px solid Black Cherry outline, 2px offset, on every interactive element.
- **Back link:** a Muted Ink 600 0.875rem text link with a leading arrow that darkens to Black Cherry on hover; used above create and join.
- **Two-step destructive action:** "Leave campaign" is a quiet pill; pressing it swaps in a Cool Blue notice naming the consequence with "Stay" (pill) and "Leave" (primary pill).

### Chips
- **Rank chip:** 2rem tall, 6px radius, 2px Black Cherry outline, Bungee 0.9375rem tabular numeral. Your row's chip fills Tangerine.
- **FINISHER tag:** Cool Blue fill, Black Cherry Bungee 0.6875rem capitals, 4px radius.

### Cards / Containers
- **Corner Style:** 10px.
- **Background:** Paper; notices fill Cool Blue.
- **Shadow Strategy:** none, except the front bib (see Elevation & Depth).
- **Border:** 2px Black Cherry, solid; dashed for the private-goal box.
- **Internal Padding:** 0.6 to 0.9rem.

### Inputs / Fields
- **Style:** Paper fill, 2px Black Cherry outline, 6px radius, min-height 2.75rem, Hanken Grotesk 400 1rem with tabular figures, 0.5rem 0.75rem padding; Muted Ink placeholder. Number spinners are hidden. A target field sets its value in Bungee 1.625rem, max 12rem wide.
- **Field:** label or legend in Hanken 700 0.9375rem above the control, 0.35rem gap, 1.25rem below; hints in Meta.
- **Focus:** the standard 3px Black Cherry ring at 2px offset.
- **Error:** the outline thickens by a 1px Black Cherry inset (reads as 3px without shifting layout) and a Black Cherry 700 0.875rem message sits under the field. No red: errors are carried by weight and wording.
- **Choice cards:** radio options as 2px-outlined 10px cards (0.7rem 0.9rem) with a bold title and Meta explanation; checked fills Cool Blue, hover fills Cool Blue Soft only where hover exists; the native radio is tinted Black Cherry; focus rings the whole card.
- **Choice pills:** short radio sets (metric) as outlined pills with the same checked and hover fills.

### Navigation
A Paper bar with a 2px Black Cherry bottom rule. Bungee wordmark left; links in Hanken Grotesk 600, Muted Ink, the active link in Black Cherry with a 3px Tangerine underline offset 6px. Logout is an outlined pill that fills Cool Blue on hover. Under 480px the user name hides; under 360px gaps and padding tighten. Campaign tabs follow the same grammar: 700 1rem labels over a 2px Hairline baseline, the selected tab marked by a 3px Tangerine underline, arrow keys move between tabs.

### The Bib (signature)
A 2px-outlined, 10px card with three parts. The **band** runs across the top in Bungee label capitals with two justified slots (campaign name and wearer; or "AHEAD" and the gap): Tangerine on the front bib, Cool Blue on peeks and when the wearer has finished ("FINISHER"). The **body** sets the rank as a monumental numeral bottom-left against the value and sub-lines right-aligned. The **tear-off tab** sits under a 2px dashed rule on Cool Blue, carrying the next gap in Hanken 700 with a trailing arrow; it is a button when it steps the field (hover deepens the blue). Peek bibs are 64% wide (max 250px), carry a 2.75rem numeral with name and last-active note, and step the field when tapped. Every swipe has the Ahead / Behind pill equivalent. A not-joined viewer sees an unclaimed bib ("No. —") holding the Join button; a pooled campaign shows one group bib with the percentage as numeral.

The **flat bib** is the same anatomy without the offset, for previews and empty states: create's live preview, join's preview with the "No. —" / "Assigned when you join" claim pair, and the empty campaigns list ("No. —" / "Unclaimed" band holding "Not racing yet" and a primary pill). When the bib links somewhere, its campaign name is a stretched link: the name carries the accessible name, the whole bib is the hit area, hover fills Cool Blue Soft, and the focus ring wraps the bib.

### The Start List (signature)
One 2px-outlined, 10px sheet per race state, no offset. Each opens with a full-width band in Label capitals with the state left and a count right: Tangerine for "Start list · Racing", Cool Blue for "On the start line" and "Results". The Racing sheet adds a column-label row (NO. / RACE / NEXT GAP). Rows put your number in Bungee (rank, group percentage, or days to start such as "12D"), the race name in Hanken 700 with a Meta line under it, and the gap or value right-aligned in Hanken 700 tabular with a Meta sub-line ("470 m" / "to Sam"). The race name is a stretched link covering the row, so hover fills the row Cool Blue Soft and the focus ring sits inset on the row; Join pills and retry links sit above it. Racing closes with a dashed-rule Cool Blue tear-off footer naming your closest move across every race, linking to that race with a trailing arrow (Cool Blue Deep on hover). A finished target adds the FINISHER tag under the race name.

### Charts
ECharts cannot read CSS variables, so charts mirror the tokens as constants. Your line is Tangerine at 4px; other members alternate Black Cherry and Muted Ink at 2px and differ by dash pattern, each named at its line end, so colour is never the only cue. Grid lines are Hairline; the target line is a 2px dashed Black Cherry rule labelled "Target". A screen-reader table carries the full data.

## Do's and Don'ts

### Do:
- **Do** put the next gap on the bib's tear-off tab ("8.2 km to catch Sam").
- **Do** set every number in tabular figures.
- **Do** give every swipe interaction a visible button equivalent ("Ahead" / "Behind").
- **Do** show "Powered by Strava" on every screen that displays Strava data.
- **Do** mark "you" with Tangerine, consistently, on every surface: your band, your rank chip, your chart line.
- **Do** outline bibs, sheets, chips, pills, inputs and notices in 2px Black Cherry, and focus with a 3px Black Cherry ring at 2px offset.
- **Do** make a whole bib or start-list row clickable with a stretched link on its name, keeping nested controls above it.
- **Do** drop trailing zeros in values: "536 km" not "536.0 km", "2h" not "2h 0m".

### Don't:
- **Don't** set text in Tangerine on white, or white text on Tangerine.
- **Don't** set people's names, activity titles or other user-written reading text in Bungee.
- **Don't** introduce green, or Strava orange `#fc4c02`, into Fitter's own UI.
- **Don't** drift into a dark neon fitness-tracker look or a generic white-card dashboard.
- **Don't** add texture, gradients or soft shadows. Depth is outline plus the single front-bib offset.
- **Don't** build new surfaces on the legacy scaffold tokens (`--accent`, `--bg`, `.card`, `.btn`).
