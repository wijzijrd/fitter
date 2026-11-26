import { describe, expect, it } from "vitest";
import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { BlockSchema, StationSchema } from "../gen/fitter/v1/activity_pb";
import { FeedActivitySchema, type FeedActivity } from "../gen/fitter/v1/feed_pb";
import { allDetailsOn, detailParts, groupByDay, isNew, liftLines, paceOrSpeed, pulse, recent } from "./feed";

const NOW = new Date(2026, 10, 22, 12, 0, 0); // Sun 22 Nov, local noon
const ago = (hours: number) => new Date(NOW.getTime() - hours * 3_600_000);
const act = (init: MessageInitShape<typeof FeedActivitySchema> & { at: Date }): FeedActivity => {
  const { at, ...rest } = init;
  return create(FeedActivitySchema, { userId: "u2", userName: "Sam", startDate: timestampFromDate(at), movingTimeS: 1800n, ...rest });
};

describe("recent", () => {
  it("keeps only the last seven days", () => {
    const rows = [act({ at: ago(1) }), act({ at: ago(24 * 6) }), act({ at: ago(24 * 8) })];
    expect(recent(rows, NOW.getTime())).toHaveLength(2);
  });
});

describe("isNew", () => {
  it("needs a previous look, someone else's row, and a start after it", () => {
    const row = act({ at: ago(1) });
    expect(isNew(row, ago(2), "u1")).toBe(true);
    expect(isNew(row, undefined, "u1")).toBe(false); // first visit: nothing is "new"
    expect(isNew(row, ago(0.5), "u1")).toBe(false); // seen after it started
    expect(isNew(row, ago(2), "u2")).toBe(false); // your own never counts
  });
});

describe("groupByDay", () => {
  it("labels days and keeps order", () => {
    const rows = [act({ at: ago(1) }), act({ at: ago(3) }), act({ at: ago(20) }), act({ at: ago(24 * 3) })];
    const groups = groupByDay(rows, NOW);
    expect(groups.map((g) => g.label.split(",")[0])).toEqual(["Today", "Yesterday", groups[2].label.split(",")[0]]);
    expect(groups.map((g) => g.rows.length)).toEqual([2, 1, 1]);
    expect(groups[2].label).not.toMatch(/Today|Yesterday/);
  });
});

describe("paceOrSpeed", () => {
  it("gives pace for runs and walks, speed for rides, nothing else", () => {
    expect(paceOrSpeed(act({ at: NOW, sportGroup: "run", distanceM: 5000, movingTimeS: 1560n }))).toBe("5:12 /km");
    expect(paceOrSpeed(act({ at: NOW, sportGroup: "walk", distanceM: 3000, movingTimeS: 2400n }))).toBe("13:20 /km");
    expect(paceOrSpeed(act({ at: NOW, sportGroup: "ride", distanceM: 30000, movingTimeS: 3600n }))).toBe("30.0 km/h");
    expect(paceOrSpeed(act({ at: NOW, sportGroup: "swim", distanceM: 1000, movingTimeS: 1200n }))).toBe("");
    expect(paceOrSpeed(act({ at: NOW, sportGroup: "run", distanceM: 0, movingTimeS: 1200n }))).toBe("");
    expect(paceOrSpeed(act({ at: NOW, sportGroup: "strength", distanceM: 0 }))).toBe("");
  });
});

describe("detailParts", () => {
  const ride = act({ at: NOW, sportGroup: "ride", distanceM: 30000, movingTimeS: 3600n, averageWatts: 212.4, sufferScore: 61.2 });
  it("shows what is on and skips what is missing", () => {
    expect(detailParts(ride, allDetailsOn)).toEqual(["30.0 km/h", "212 W", "Effort 61"]);
    expect(detailParts(act({ at: NOW, sportGroup: "strength" }), allDetailsOn)).toEqual([]);
  });
  it("hides what the viewer turned off", () => {
    expect(detailParts(ride, { ...allDetailsOn, showPace: false, showEffort: false })).toEqual(["212 W"]);
  });
});

describe("liftLines", () => {
  const block = (name: string) => create(BlockSchema, { rounds: 1, stations: [create(StationSchema, { exerciseName: name, reps: 8 })] });
  it("cuts a long session to three lines and counts the rest", () => {
    expect(liftLines([block("A"), block("B")])).toEqual(["A × 8", "B × 8"]);
    expect(liftLines(["A", "B", "C", "D", "E"].map(block))).toEqual(["A × 8", "B × 8", "C × 8", "+2 more"]);
  });
});

describe("pulse", () => {
  const rows = (...n: string[]) => n.map((userName) => act({ at: ago(1), userName }));
  it("names who logged what", () => {
    expect(pulse([])).toBe("");
    expect(pulse(rows("Sam"))).toBe("Sam logged 1 activity since you last looked.");
    expect(pulse(rows("Sam", "Sam", "Jo"))).toBe("Sam and Jo logged 3 activities since you last looked.");
    expect(pulse(rows("Sam", "Jo", "Lee", "Ana", "Ana", "Ana"))).toBe("Sam, Jo and 2 others logged 6 activities since you last looked.");
  });
});
