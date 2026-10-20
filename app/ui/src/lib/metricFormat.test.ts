import { describe, expect, it } from "vitest";
import { ActivityMetric, MetricUnit } from "../gen/fitter/v1/common_pb";
import { formatMetricValue } from "./metricFormat";

describe("formatMetricValue", () => {
  it.each([
    [MetricUnit.METERS, 536_000, undefined, "536 km"],
    [MetricUnit.METERS, 62_340, undefined, "62.3 km"],
    [MetricUnit.METERS, 0, undefined, "0 km"],
    [MetricUnit.METERS, 850, undefined, "850 m"],
    [MetricUnit.METERS, 1234, ActivityMetric.ELEVATION_GAIN, "1,234 m"],
    [MetricUnit.SECONDS, 7200, undefined, "2h"],
    [MetricUnit.SECONDS, 45_000, undefined, "12h 30m"],
    [MetricUnit.SECONDS, 2699, undefined, "44:59"],
    [MetricUnit.SECONDS_PER_KM, 275, undefined, "4:35/km"],
    [MetricUnit.METERS_PER_SECOND, 10, undefined, "36 km/h"],
    [MetricUnit.BPM, 151.6, undefined, "152 bpm"],
    [MetricUnit.COUNT, 1, ActivityMetric.ACTIVITY_COUNT, "1 activity"],
    [MetricUnit.COUNT, 12, ActivityMetric.ACTIVITY_COUNT, "12 activities"],
  ])("unit %i value %d → %s", (unit, value, metric, want) => {
    expect(formatMetricValue(unit, value, metric)).toBe(want);
  });
});
