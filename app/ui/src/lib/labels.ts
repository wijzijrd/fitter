import { ActivityMetric, ClauseScope, Comparator } from "../gen/fitter/v1/common_pb";
import type { SportSelector } from "../gen/fitter/v1/common_pb";
import type { MetricInfo, SportGroupInfo } from "../gen/fitter/v1/catalog_pb";
import type { FeedActivity } from "../gen/fitter/v1/feed_pb";
import type { GoalClause } from "../gen/fitter/v1/goal_pb";

// ClauseScope and Comparator are small, stable protocol-level enums (not
// part of the 17-metric catalog), so they stay as plain maps here rather
// than coming from CatalogService.
export const scopeLabels: Record<ClauseScope, string> = {
  [ClauseScope.UNSPECIFIED]: "Unspecified",
  [ClauseScope.AGGREGATE]: "Total",
  [ClauseScope.SINGLE]: "One activity",
};

export const comparatorLabels: Record<Comparator, string> = {
  [Comparator.UNSPECIFIED]: "Unspecified",
  [Comparator.GTE]: "at least",
  [Comparator.LTE]: "at most",
};

// sportSelectorLabel renders a SportSelector oneof for display, using the
// sport-group catalog to turn a group key into its human label.
export function sportSelectorLabel(sport: SportSelector | undefined, sportGroups: SportGroupInfo[]): string {
  if (!sport || sport.selector.case === undefined) return "All sports";
  if (sport.selector.case === "group") {
    const group = sportGroups.find((g) => g.group === sport.selector.value);
    return group ? group.label : sport.selector.value;
  }
  return sport.selector.value;
}

// For lower-is-better metrics (pace) "at most 4:45/km" means faster, which
// reads backwards; say it the way runners do.
export const fasterLabels: Record<Comparator, string> = {
  [Comparator.UNSPECIFIED]: "Unspecified",
  [Comparator.GTE]: "or slower",
  [Comparator.LTE]: "or faster",
};

type ClauseDef = Pick<GoalClause, "scope" | "metric" | "comparator" | "value">;

// clauseText reads a clause as a sentence fragment: "Distance at least 100 km",
// "Pace 4:35/km or faster in one activity".
export function clauseText(
  c: ClauseDef,
  info: MetricInfo | undefined,
  fmt: (metric: ActivityMetric, v: number) => string,
): string {
  const scope = c.scope === ClauseScope.SINGLE ? " in one activity" : "";
  const label = info?.label ?? "Unknown metric";
  const value = fmt(c.metric, c.value);
  return info?.lowerIsBetter
    ? `${label} ${value} ${fasterLabels[c.comparator]}${scope}`
    : `${label} ${comparatorLabels[c.comparator]} ${value}${scope}`;
}

const ordinalRules = new Intl.PluralRules("en", { type: "ordinal" });
const ordinalSuffix: Record<string, string> = { one: "st", two: "nd", few: "rd", other: "th" };

export function ordinal(n: number): string {
  return `${n}${ordinalSuffix[ordinalRules.select(n)]}`;
}

// activityMetricValue extracts the raw value of a single feed activity for
// the given metric, or undefined when the feed doesn't carry that metric.
export function activityMetricValue(a: FeedActivity, metric: ActivityMetric): number | undefined {
  switch (metric) {
    case ActivityMetric.DISTANCE:
      return a.distanceM;
    case ActivityMetric.ELEVATION_GAIN:
      return a.elevationM;
    case ActivityMetric.MOVING_TIME:
      return Number(a.movingTimeS);
    case ActivityMetric.ACTIVITY_COUNT:
      return 1;
    case ActivityMetric.AVERAGE_HEARTRATE:
      return a.averageHeartrate;
    case ActivityMetric.AVERAGE_WATTS:
      return a.averageWatts;
    case ActivityMetric.SUFFER_SCORE:
      return a.sufferScore;
    default:
      return undefined;
  }
}
