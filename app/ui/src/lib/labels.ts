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

// formatMetricValue renders a raw metric value (meters, seconds, or a plain
// count) in units people actually read.
export function formatMetricValue(metric: ActivityMetric, value: number): string {
  switch (metric) {
    case ActivityMetric.DISTANCE:
      return `${(value / 1000).toFixed(1)} km`;
    case ActivityMetric.ELEVATION:
      return `${Math.round(value)} m`;
    case ActivityMetric.TIME: {
      const hours = Math.floor(value / 3600);
      const minutes = Math.round((value % 3600) / 60);
      return `${hours}h ${minutes}m`;
    }
    case ActivityMetric.COUNT:
      return `${Math.round(value)} activities`;
    default:
      return `${value}`;
  }
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
