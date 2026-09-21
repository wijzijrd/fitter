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
