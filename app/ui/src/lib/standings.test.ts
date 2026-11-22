import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { ActivityMetric, ClauseScope, Comparator } from "../gen/fitter/v1/common_pb";
import { LeaderboardEntrySchema, LeaderboardGroupSchema } from "../gen/fitter/v1/feed_pb";
import { amount, gap, lead, myEntries, peekBand, pickGroup, primary, share, unitClause } from "./standings";

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

describe("pickGroup", () => {
  it("opens on your best standing even when the all-sports key is empty", () => {
    expect(pickGroup(["", "ride"], null, "ride")).toBe("ride");
  });

  it("keeps the current group, including the empty all-sports key", () => {
    expect(pickGroup(["", "ride"], "", "ride")).toBe("");
    expect(pickGroup(["", "ride"], "ride", undefined)).toBe("ride");
  });

  it("falls back to the first group, then to nothing", () => {
    expect(pickGroup(["__together__", ""], null, undefined)).toBe("__together__");
    expect(pickGroup(["run"], "gone", undefined)).toBe("run");
    expect(pickGroup([], "gone", undefined)).toBeNull();
  });
});

describe("share", () => {
  it("floors to a whole percent and survives float noise", () => {
    expect(share(39, 201)).toBe(19);
    expect(share(58, 100)).toBe(58);
    expect(share(0.58 * 100, 100)).toBe(58);
  });

  it("is 0 before anything is pooled", () => {
    expect(share(5, 0)).toBe(0);
  });
});

describe("peekBand: the gap on a neighbouring bib is measured against the front bib", () => {
  const kmFmt = fmt;
  it("names who the difference is against, so it can't be read as the neighbour's own shortfall", () => {
    // The beta report: 66% in front, 9% behind, shown as "Behind −57%" with nothing to say against whom.
    const me = entry("me", "g1", 0.66, 1, [{ scope: ClauseScope.AGGREGATE, metric: ActivityMetric.MOVING_TIME, comparator: Comparator.GTE, value: 1 }]);
    const anre = entry("anre", "g2", 0.09, 2);
    expect(peekBand("behind", me, anre, kmFmt, true)).toEqual(["Behind", "−57% vs you"]);
    expect(peekBand("ahead", anre, me, kmFmt, false)).toEqual(["Ahead", "+57% vs anre"]);
  });
  it("uses units only when both bibs chase the same metric and target", () => {
    const a = entry("me", "g1", 0.7, 1);
    const b = entry("sam", "g2", 0.2, 2);
    expect(peekBand("behind", a, b, kmFmt, true)[1]).toBe("−50 km vs you");
    const other = entry("sam", "g3", 0.2, 2, [distance(200_000)]);
    expect(peekBand("behind", a, other, kmFmt, true)[1]).toBe("−50% vs you");
  });
  it("is Level when both are the same, including two finishers past their goals", () => {
    const a = entry("me", "g1", 1, 1);
    const b = entry("sam", "g2", 1, 2);
    expect(peekBand("behind", a, b, kmFmt, true)).toEqual(["Behind", "Level with you"]);
    expect(peekBand("ahead", b, a, kmFmt, false)).toEqual(["Ahead", "Level with sam"]);
  });
  it("shows one point of difference between 99% and a finisher, never a negative or over-100 figure", () => {
    const fin = entry("me", "g1", 1, 1);
    const near = entry("sam", "g2", 0.99, 2, [{ scope: ClauseScope.AGGREGATE, metric: ActivityMetric.MOVING_TIME, comparator: Comparator.GTE, value: 1 }]);
    expect(peekBand("behind", fin, near, kmFmt, true)[1]).toBe("−1% vs you");
  });
});

describe("goals beyond 100%", () => {
  it("shows the real total in units but the share is capped, so a finisher at 130 km reads 130 km", () => {
    const over = create(LeaderboardEntrySchema, {
      userId: "me", userName: "Me", goalId: "g", progressRatio: 1, rank: 1, satisfied: true,
      clauseDefs: [distance(100_000)], clauses: [{ currentValue: 130_000, progressRatio: 1 }],
    });
    expect(primary(over, fmt)).toBe("130 km");
    const pctOnly = entry("me", "g", 1, 1, [distance(100_000), distance(1)]);
    expect(primary(pctOnly, fmt)).toBe("100%");
  });
  it("two finishers are level, and a finisher alone has nothing left to chase", () => {
    const g = group("run", entry("a", "g1", 1, 1), entry("me", "g2", 1, 2));
    expect(gap(g, g.entries[1], fmt)).toEqual({ kind: "level", other: g.entries[0] });
    const solo = group("run", entry("me", "g3", 1, 1));
    expect(gap(solo, solo.entries[0], fmt)).toEqual({ kind: "done" });
  });
  it("counts a finisher's share as 100 whatever the ratio", () => {
    expect(share(1, 1)).toBe(100);
    expect(share(0, 1)).toBe(0);
    expect(share(5, 0)).toBe(0);
  });
});
