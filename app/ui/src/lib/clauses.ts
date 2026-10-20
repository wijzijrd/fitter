import { ActivityMetric, ClauseScope, Comparator } from "../gen/fitter/v1/common_pb";

// ClauseInput is the shape ClauseBuilder.vue works with — structurally
// compatible with the MessageInitShape connect-es accepts for
// fitter.v1.GoalClause on CreateGoal/UpdateGoal/JoinCampaign requests (id
// and position are server-assigned/server-computed, so callers never need
// to set them).
export interface ClauseInput {
  scope: ClauseScope;
  metric: ActivityMetric;
  comparator: Comparator;
  value: number;
}

// additiveMetrics add up across activities (SUM/COUNT aggregation in
// internal/metrics), so progress and gaps read naturally in their units.
// Averages, maxima and pace don't: 50% of a pace target isn't a pace.
export const additiveMetrics: ReadonlySet<ActivityMetric> = new Set([
  ActivityMetric.DISTANCE,
  ActivityMetric.ELEVATION_GAIN,
  ActivityMetric.MOVING_TIME,
  ActivityMetric.ELAPSED_TIME,
  ActivityMetric.ACTIVITY_COUNT,
  ActivityMetric.KILOJOULES,
  ActivityMetric.CALORIES,
  ActivityMetric.SUFFER_SCORE,
]);
