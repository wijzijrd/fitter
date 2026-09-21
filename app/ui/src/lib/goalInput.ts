import type { ClauseInput } from "./clauses";
import type { SportSelectorValue } from "./sportSelector";
import { sportSelectorInit } from "./sportSelector";
import { fromDate } from "./timestamp";

// GoalFormInput is the shared shape edited by the personal-goal create form
// and the join-campaign flow's per-goal sub-form — both collect a name, a
// sport selector, and one or more clauses via ClauseBuilder.
export interface GoalFormInput {
  name: string;
  sport: SportSelectorValue;
  periodStart?: Date;
  periodEnd?: Date;
  clauses: ClauseInput[];
}

// toCreateGoalInit builds the plain init object CreateGoal/JoinCampaign
// expects. period is omitted when absent — the server forces it to the
// campaign window whenever campaignId is set (join flow, or a personal goal
// pointed at a campaign), so the join flow never needs to supply one.
export function toCreateGoalInit(input: GoalFormInput, campaignId?: string) {
  return {
    campaignId,
    name: input.name,
    sport: sportSelectorInit(input.sport),
    periodStart: input.periodStart ? fromDate(input.periodStart) : undefined,
    periodEnd: input.periodEnd ? fromDate(input.periodEnd) : undefined,
    clauses: input.clauses,
  };
}
