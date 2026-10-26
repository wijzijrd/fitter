import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { ActivityMetric, ClauseScope, Comparator } from "../gen/fitter/v1/common_pb";
import { LeaderboardEntrySchema, LeaderboardGroupSchema } from "../gen/fitter/v1/feed_pb";
import { amount, gap, lead, myEntries, primary, unitClause } from "./standings";

const fmt = (_m: ActivityMetric, v: number) => `${Math.round(v / 1000)} km`;
const distance = (target: number) => ({
  scope: ClauseScope.AGGREGATE,
  metric: ActivityMetric.DISTANCE,
  comparator: Comparator.GTE,
  value: target,
});

function entry(userId: string, goalId: string, ratio: number, rank: number, clauseDefs = [distance(100_000)]) {
  return create(LeaderboardEntrySchema, {
    userId,
    userName: `${userId} Surname`,
    goalId,
    progressRatio: ratio,
    rank,
    satisfied: ratio >= 1,
    clauseDefs,
    clauses: clauseDefs.map((c) => ({ currentValue: ratio * c.value, progressRatio: ratio })),
  });
}

const group = (key: string, ...entries: ReturnType<typeof entry>[]) =>
  create(LeaderboardGroupSchema, { sportGroup: key, entries });

describe("standings", () => {
  const run = group("run", entry("sam", "g1", 0.8, 1), entry("me", "g2", 0.5, 2), entry("ana", "g3", 0.2, 3));
  const ride = group("ride", entry("me", "g0", 0.9, 1), entry("ana", "g4", 0.4, 2));

  it("finds my entries across groups and leads with the best rank", () => {
    const mine = myEntries([run, ride], "me");
    expect(mine.map((p) => p.entry.goalId)).toEqual(["g2", "g0"]);
    expect(lead(mine)?.entry.goalId).toBe("g0");
  });

  it("breaks rank ties by ratio, then goal id", () => {
    const a = { group: run, entry: entry("me", "b", 0.5, 1) };
    const b = { group: run, entry: entry("me", "a", 0.5, 1) };
    const c = { group: run, entry: entry("me", "c", 0.6, 1) };
    expect(lead([a, b, c])?.entry.goalId).toBe("c");
    expect(lead([a, b])?.entry.goalId).toBe("a");
  });

  it("measures gaps in units for a single total clause", () => {
    const me = run.entries[1];
    expect(unitClause(me)).toBeDefined();
    expect(primary(me, fmt)).toBe("50 km");
    expect(gap(run, me, fmt)).toMatchObject({ kind: "chase", amount: "30 km", closeness: expect.closeTo(0.3) });
    expect(gap(run, run.entries[0], fmt)).toMatchObject({ kind: "clear", amount: "30 km" });
  });

  it("falls back to percentages for multi-clause goals", () => {
    const e = entry("me", "g9", 0.42, 1, [distance(10_000), { ...distance(5000), scope: ClauseScope.SINGLE }]);
    expect(unitClause(e)).toBeUndefined();
    expect(primary(e, fmt)).toBe("42%");
    expect(amount(e, 0.1, fmt)).toBe("10%");
    expect(gap(group("", e), e, fmt)).toEqual({ kind: "finish", amount: "58%" });
  });

  it("uses percentage points when the other goal has a different target or metric", () => {
    const g = group("", entry("sam", "1", 0.66, 1, [distance(300_000)]), entry("me", "2", 0.6, 2, [distance(350_000)]));
    expect(gap(g, g.entries[1], fmt)).toMatchObject({ kind: "chase", amount: "6%" });
    const pace = { scope: ClauseScope.AGGREGATE, metric: ActivityMetric.PACE, comparator: Comparator.GTE, value: 300 };
    const p = entry("me", "3", 0.5, 1, [pace]);
    expect(unitClause(p)).toBeUndefined();
    expect(primary(p, fmt)).toBe("50%");
  });

  it("measures against other people, not your own other goals", () => {
    const g = group("", entry("me", "1", 0.9, 1), entry("me", "2", 0.9, 2), entry("ana", "3", 0.5, 3));
    expect(gap(g, g.entries[0], fmt)).toMatchObject({ kind: "clear", other: { userId: "ana" } });
    expect(gap(g, g.entries[1], fmt)).toMatchObject({ kind: "clear", other: { userId: "ana" } });
  });

  it("reports level and done", () => {
    const g = group("", entry("a", "1", 1, 1), entry("me", "2", 1, 2));
    expect(gap(g, g.entries[1], fmt).kind).toBe("level");
    const solo = group("", entry("me", "3", 1, 1));
    expect(gap(solo, solo.entries[0], fmt).kind).toBe("done");
  });
});
