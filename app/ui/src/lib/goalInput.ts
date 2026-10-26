import { ActivityMetric, ClauseScope, Comparator, MetricUnit } from "../gen/fitter/v1/common_pb";
import type { ClauseInput } from "./clauses";
import { additiveMetrics } from "./clauses";
import { baseValueToEditString, editStringToBaseValue } from "./metricFormat";
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

// GoalDraft is what GoalBuilder edits: clause values stay as typed text
// ("42.2", "4:35") until the draft is converted, since parsing depends on
// each clause's metric unit.
export interface ClauseDraft {
  key: number;
  scope: ClauseScope;
  metric: ActivityMetric;
  comparator: Comparator;
  valueText: string;
}

export interface GoalDraft {
  name: string;
  sport: SportSelectorValue;
  clauses: ClauseDraft[];
}

type UnitOf = (metric: ActivityMetric) => MetricUnit;

let nextKey = 0;

export function blankClause(metric = ActivityMetric.DISTANCE): ClauseDraft {
  return { key: nextKey++, scope: ClauseScope.AGGREGATE, metric, comparator: Comparator.GTE, valueText: "" };
}

export function blankDraft(): GoalDraft {
  return { name: "", sport: { case: undefined }, clauses: [blankClause()] };
}

export function clauseValue(c: ClauseDraft, unitOf: UnitOf): number | null {
  return editStringToBaseValue(unitOf(c.metric), c.valueText);
}

// draftErrors returns "" for each part that's valid.
export function draftErrors(d: GoalDraft, unitOf: UnitOf): { name: string; clauses: string[]; any: boolean } {
  const name = d.name.trim() ? "" : "Name the goal.";
  const clauses = d.clauses.map((c) => {
    if (!c.valueText.trim()) return "Enter a value.";
    const v = clauseValue(c, unitOf);
    return v === null || !(v > 0) ? "Enter a value above zero." : "";
  });
  return { name, clauses, any: !!name || clauses.some(Boolean) || !d.clauses.length };
}

export function draftToInput(d: GoalDraft, unitOf: UnitOf): GoalFormInput {
  return {
    name: d.name.trim(),
    sport: d.sport,
    clauses: d.clauses.map((c) => ({
      scope: c.scope,
      metric: c.metric,
      comparator: c.comparator,
      value: clauseValue(c, unitOf) ?? 0,
    })),
  };
}

// inputToDraft starts a draft from an existing goal (editing, or copying a
// friend's goal off the leaderboard).
export function inputToDraft(
  g: { name: string; sport: SportSelectorValue; clauses: ClauseInput[] },
  unitOf: UnitOf,
): GoalDraft {
  return {
    name: g.name,
    sport: g.sport,
    clauses: g.clauses.map((c) => ({
      key: nextKey++,
      scope: c.scope,
      metric: c.metric,
      comparator: c.comparator,
      valueText: baseValueToEditString(unitOf(c.metric), c.value),
    })),
  };
}

// unitDraft mirrors standings' unitClause: a single "total at least" condition reads in units.
export function unitDraft(d: GoalDraft): ClauseDraft | undefined {
  const [only, ...rest] = d.clauses;
  return only &&
    !rest.length &&
    only.scope === ClauseScope.AGGREGATE &&
    only.comparator === Comparator.GTE &&
    additiveMetrics.has(only.metric)
    ? only
    : undefined;
}
