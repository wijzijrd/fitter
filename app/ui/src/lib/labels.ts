import { ActivityMetric, CampaignMode } from "../gen/fitter/v1/common_pb";
import type { FeedActivity } from "../gen/fitter/v1/feed_pb";

export const metricLabels: Record<ActivityMetric, string> = {
  [ActivityMetric.UNSPECIFIED]: "Unspecified",
  [ActivityMetric.DISTANCE]: "Distance",
  [ActivityMetric.ELEVATION]: "Elevation gain",
  [ActivityMetric.TIME]: "Moving time",
  [ActivityMetric.COUNT]: "Activity count",
};

export const modeLabels: Record<CampaignMode, string> = {
  [CampaignMode.UNSPECIFIED]: "Unspecified",
  [CampaignMode.POOLED]: "Pooled (shared target)",
  [CampaignMode.PER_MEMBER]: "Per-member (own target)",
};

// Targets are stored in raw units (metres, seconds) but typed in the units people
// think in; `factor` converts the typed number to the stored one.
export const targetInputUnits: Record<ActivityMetric, { unit: string; factor: number }> = {
  [ActivityMetric.UNSPECIFIED]: { unit: "", factor: 1 },
  [ActivityMetric.DISTANCE]: { unit: "km", factor: 1000 },
  [ActivityMetric.ELEVATION]: { unit: "m", factor: 1 },
  [ActivityMetric.TIME]: { unit: "minutes", factor: 60 },
  [ActivityMetric.COUNT]: { unit: "activities", factor: 1 },
};

const oneDecimal = new Intl.NumberFormat("en", { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const whole = new Intl.NumberFormat("en", { maximumFractionDigits: 0 });

// formatMetricValue renders a raw metric value (meters, seconds, or a plain
// count) in units people actually read.
export function formatMetricValue(metric: ActivityMetric, value: number): string {
  switch (metric) {
    case ActivityMetric.DISTANCE: {
      // Whole kilometres drop the ".0": "536 km", but "62.3 km".
      const km = Math.round(value / 100) / 10;
      return `${Number.isInteger(km) ? whole.format(km) : oneDecimal.format(km)} km`;
    }
    case ActivityMetric.ELEVATION:
      return `${whole.format(value)} m`;
    case ActivityMetric.TIME: {
      const hours = Math.floor(value / 3600);
      const minutes = Math.round((value % 3600) / 60);
      if (!minutes) return `${hours}h`;
      return hours ? `${hours}h ${minutes}m` : `${minutes}m`;
    }
    case ActivityMetric.COUNT:
      return `${whole.format(value)} activities`;
    default:
      return `${value}`;
  }
}

const ordinalRules = new Intl.PluralRules("en", { type: "ordinal" });
const ordinalSuffix: Record<string, string> = { one: "st", two: "nd", few: "rd", other: "th" };

export function ordinal(n: number): string {
  return `${n}${ordinalSuffix[ordinalRules.select(n)]}`;
}

// activityMetricValue extracts the raw value of a single feed activity for
// the given metric — "count" is always 1 per activity.
export function activityMetricValue(activity: FeedActivity, metric: ActivityMetric): number {
  switch (metric) {
    case ActivityMetric.DISTANCE:
      return activity.distanceM;
    case ActivityMetric.ELEVATION:
      return activity.elevationM;
    case ActivityMetric.TIME:
      return Number(activity.movingTimeS);
    case ActivityMetric.COUNT:
      return 1;
    default:
      return 0;
  }
}
