---
target: campaign detail (CampaignDetailView.vue)
total_score: 27
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 4
target_identity: "file:/Users/tauriqbehardien/repositories/fitter/app/ui/src/views/CampaignDetailView.vue"
target_fingerprint: "sha256:cc9f54c5700974778cace8d186bc14fd820fbeff39e0fd51933dde2db60b1eb7"
target_path: /Users/tauriqbehardien/repositories/fitter/app/ui/src/views/CampaignDetailView.vue
timestamp: 2026-10-26T15-04-14Z
slug: app-ui-src-views-campaigndetailview-vue
closed: true
---
Method: dual-agent (A: a86c53b4ed97bff7d · B: ac40ed40f814828a6)

# Critique: campaign detail (Operate), 27/40

Scores: H1 3, H2 3, H3 3, H4 3, H5 3, H6 2, H7 2, H8 3, H9 3, H10 2. Total 27/40, Good.

Design specificity: authored for Fitter in the first 360px (Bib Stack); generic below it (standings rows, feed list, default ECharts timeline).

Deterministic scan: CLI 0 findings. Live DOM Timeline tab: 6 text-occlusion (false positives, hidden data table) and 1 skipped-heading (real).

## Priority issues
- [P1] Default bib selection wrong when All-sports key is "" (BibStack.vue:33-41). Pooled never opens on Together; "Back to you" beside own bib. Fix: non-empty sentinel or selected=null. Command: harden.
- [P1] Row taps act off-screen, focus drops to body after stepping, three controls step the same way (CampaignDetailView.vue:253, BibStack.vue). Fix: row affordance + selected state, scroll stack into view, move focus, aria-disabled. Command: harden.
- [P1] Timeline tab scrolls sideways (+79px at 390, +149px at 320): visually-hidden table in TimelineChart.vue:184,202-210 keeps content width. Also h1 to h3 heading skip, no h2. Fix: wrap table in overflow-hidden 1px box, dedupe .visually-hidden (style.css:280, BibStack.vue:473, TimelineChart.vue:202), group labels as h2. Command: harden.
- [P1/P2] Pooled bib drifted from confirmed decision: no contributor peeks, tab says "to go together" not your contribution, duplicate Together row, rows rank by goal progress. Command: layout.
- [P2] Finished campaign behaves live: "19 km to catch Priya", "Add a goal" still shown, no result moment. Command: delight.

## Minor
- [P2] "YOU" truncates to "Y": BibStack .band > span lacks flex:none (CampaignBib.vue:74 has it).
- a.back 91x18 fails WCAG 2.2 target size; Feed tab 35px wide, tabs 42px tall.
- Loading state is bare text; no skeleton bib.
- Band name slot duplicates h1.

## Personas
Sam, Casey, Riley, and a friend checking after a workout: see chat report.

## Questions
Rank movement dropped: what signals change since last look? Finished campaign deserves its own result artefact? Which of five stepping interactions is the real one? Pooled default front should be the group bib?
