---
target: login
total_score: 19
max_score: 32
na_heuristics: 7,10
p0_count: 1
p1_count: 2
target_identity: "file:/Users/tauriqbehardien/repositories/fitter/app/ui/src/views/LoginView.vue"
target_fingerprint: "sha256:dab2d1c362330ae7904e5a5473ed370bd7e6d321feee3140d8abd63f4a3b98d2"
target_path: /Users/tauriqbehardien/repositories/fitter/app/ui/src/views/LoginView.vue
timestamp: 2026-10-26T16-03-32Z
slug: app-ui-src-views-loginview-vue
---
Method: dual-agent (A: ab091e14eeb65efef, B: ae986445e04a6042c)

# Critique: login

Score 19/32 (n/a: 7, 10). Acceptable.

Heuristics: 1=2, 2=3, 3=2, 4=3, 5=2, 6=3, 8=3, 9=1.

Specificity: authored (consent in tear-off tab, "No. -" band). Gap: no numeral, wordmark outranks bib title.
Detector: 0 findings on LoginView, AuthCallbackView, AuthShell and rendered routes. Overlay unavailable.

## Priority issues
- [P0] Profile-fetch failure traps user: token persisted before fetchCurrentUser; "Back to sign in" -> /login -> guard redirects authenticated users to campaigns. Fix: auth.logout() in catch or Connect link in error state. (harden)
- [P1] Backend failures dead-end: auth.go http.Error plain text for 400/502/500; errParam unescaped. Fix: redirect to /login?error=<code>, map copy, QueryEscape. (harden)
- [P1] Return-to fragile: sessionStorage lost across in-app browser / Strava app hand-off; 401 handler (lib/connect.ts:23) never saves path. Fix: localStorage with expiry or OAuth state. (harden)
- [P2] Cancel copy blames Strava; no invite context from saved path. (clarify)
- [P2] Hierarchy: wordmark > title, band Tangerine next to Strava orange, desktop 55% empty. (layout)

## Persona red flags
Jordan: no context on who invited them. Riley: "private ones too" unclear on what friends see. Sam: P0 trap.

## Minor
Callback error lacks bib; band/title duplicate; role=alert at load; title same on every route; lede widow; unused catch binding; pill 40px tall; dev .env missing.
