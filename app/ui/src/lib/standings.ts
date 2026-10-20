import { ClauseScope, Comparator } from "../gen/fitter/v1/common_pb";
import type { ActivityMetric } from "../gen/fitter/v1/common_pb";
import type { LeaderboardEntry, LeaderboardGroup } from "../gen/fitter/v1/feed_pb";
import type { GoalClause } from "../gen/fitter/v1/goal_pb";
import { additiveMetrics } from "./clauses";

// Leaderboard entries are per goal and ranked within a sport group by
// progress ratio (0–1, the minimum over the goal's clauses, with "at most"
// clauses already inverted server-side). These helpers turn that into "where
// do I stand" for the campaigns list and the bib stack.

export type Fmt = (metric: ActivityMetric, v: number) => string;

export interface Placed {
  group: LeaderboardGroup;
  entry: LeaderboardEntry;
}

export function myEntries(groups: LeaderboardGroup[], meId: string | undefined): Placed[] {
  if (!meId) return [];
  return groups.flatMap((group) => group.entries.filter((e) => e.userId === meId).map((entry) => ({ group, entry })));
}

// lead picks the standing to headline when someone has several goals: best
// rank, then further along, then the goal created first (goal IDs are UUIDv7).
export function lead(placed: Placed[]): Placed | undefined {
  return [...placed].sort(
    (a, b) =>
      a.entry.rank - b.entry.rank ||
      b.entry.progressRatio - a.entry.progressRatio ||
      (a.entry.goalId < b.entry.goalId ? -1 : a.entry.goalId > b.entry.goalId ? 1 : 0),
  )[0];
}

// unitClause is the goal's clause when it is a single "total at least" one on
// an additive metric, the only shape where progress reads naturally in units
// ("62 km") rather than as a share of the goal.
export function unitClause(e: LeaderboardEntry): GoalClause | undefined {
  const [only, ...rest] = e.clauseDefs;
  return only &&
    !rest.length &&
    only.scope === ClauseScope.AGGREGATE &&
    only.comparator === Comparator.GTE &&
    additiveMetrics.has(only.metric)
    ? only
    : undefined;
}

// The epsilon keeps float noise (1 - 0.42 = 0.57999…) from flooring a whole point away.
const pct = (ratio: number) => `${Math.floor(ratio * 100 + 1e-9)}%`;

// primary is the headline value on a bib: units for a unit clause, else the share done.
export function primary(e: LeaderboardEntry, fmt: Fmt): string {
  const u = unitClause(e);
  const p = e.clauses[0];
  return u && p ? fmt(u.metric, p.currentValue) : pct(e.progressRatio);
}

// amount renders a ratio difference in the entry's own units when it has a
// unit clause ("12 km"), otherwise in percentage points ("8%"). Against
// another entry, units only hold when both chase the same metric and target;
// otherwise "12 km to catch" would contradict the figures shown beside it.
export function amount(e: LeaderboardEntry, dRatio: number, fmt: Fmt, other?: LeaderboardEntry): string {
  const u = unitClause(e);
  const o = other && unitClause(other);
  const comparable = !other || (o && o.metric === u?.metric && o.value === u.value);
  return u && comparable ? fmt(u.metric, dRatio * u.value) : pct(dRatio);
}

export type Gap =
  | { kind: "chase"; other: LeaderboardEntry; amount: string; closeness: number }
  | { kind: "level"; other: LeaderboardEntry }
  | { kind: "clear"; other: LeaderboardEntry; amount: string }
  | { kind: "finish"; amount: string }
  | { kind: "done" };

// gap compares an entry with the one directly ahead in its group (or, when
// it leads, the one behind). closeness is the ratio gap, so gaps in
// different units and goals still compare.
export function gap(group: LeaderboardGroup, e: LeaderboardEntry, fmt: Fmt): Gap {
  const i = group.entries.findIndex((x) => x.goalId === e.goalId);
  const ahead = group.entries[i - 1];
  const behind = group.entries[i + 1];
  if (ahead) {
    const d = ahead.progressRatio - e.progressRatio;
    return d > 0 ? { kind: "chase", other: ahead, amount: amount(e, d, fmt, ahead), closeness: d } : { kind: "level", other: ahead };
  }
  if (behind) {
    const d = e.progressRatio - behind.progressRatio;
    return d > 0 ? { kind: "clear", other: behind, amount: amount(e, d, fmt, behind) } : { kind: "level", other: behind };
  }
  return e.satisfied ? { kind: "done" } : { kind: "finish", amount: amount(e, 1 - e.progressRatio, fmt) };
}

export function closeness(g: Gap): number | undefined {
  return g.kind === "chase" ? g.closeness : undefined;
}

export const firstName = (e: LeaderboardEntry) => e.userName.split(" ")[0];
