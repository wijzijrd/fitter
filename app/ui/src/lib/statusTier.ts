// statusTier maps a goal/clause/leaderboard-entry's satisfied flag and
// progress ratio to a three-tier semantic status, shared by the badge and
// progress-bar treatments across ClauseProgressList.vue, GoalsView.vue, and
// CampaignDetailView.vue's leaderboard so all three status pills (and the
// Progress fill color) agree on the same thresholds.
export type StatusTier = "success" | "warning" | "neutral";

export function statusTier(satisfied: boolean, ratio: number): StatusTier {
  if (satisfied || ratio >= 1.0) return "success";
  if (ratio >= 0.5) return "warning";
  return "neutral";
}

// badgeVariantForTier maps a StatusTier to a Badge component variant name.
export function badgeVariantForTier(tier: StatusTier): "success" | "warning" | "outline" {
  switch (tier) {
    case "success":
      return "success";
    case "warning":
      return "warning";
    default:
      return "outline";
  }
}

// progressIndicatorClassForTier maps a StatusTier to a Tailwind background
// class for Progress's fill, mirroring the badge tiers.
export function progressIndicatorClassForTier(tier: StatusTier): string {
  switch (tier) {
    case "success":
      return "bg-green-600";
    case "warning":
      return "bg-amber-500";
    default:
      return "bg-primary";
  }
}
