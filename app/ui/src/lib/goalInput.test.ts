import { describe, expect, it } from "vitest";
import { ActivityMetric, ClauseScope, MetricUnit, Recurrence } from "../gen/fitter/v1/common_pb";
import { draftErrors, draftToInput, habitPreset, stretchPreset } from "./goalInput";

const unitOf = (m: ActivityMetric) => (m === ActivityMetric.DISTANCE ? MetricUnit.METERS : MetricUnit.COUNT);

describe("presets", () => {
  it("habit repeats weekly on active days", () => {
    const { draft } = habitPreset();
    expect(draftErrors(draft, unitOf).any).toBe(false);
    const input = draftToInput(draft, unitOf);
    expect(input.recurrence).toBe(Recurrence.WEEKLY);
    expect(input.clauses[0]).toMatchObject({ metric: ActivityMetric.ACTIVE_DAYS, value: 3 });
  });
  it("stretch is a single-activity 5 km target", () => {
    const input = draftToInput(stretchPreset().draft, unitOf);
    expect(input.recurrence).toBe(Recurrence.NONE);
    expect(input.clauses[0]).toMatchObject({ scope: ClauseScope.SINGLE, metric: ActivityMetric.DISTANCE, value: 5000 });
  });
});
