import { ClauseScope, Comparator } from "../gen/fitter/v1/common_pb";
import type { SportSelector } from "../gen/fitter/v1/common_pb";
import type { SportGroupInfo } from "../gen/fitter/v1/catalog_pb";

// ClauseScope and Comparator are small, stable protocol-level enums (not
// part of the 17-metric catalog), so they stay as plain maps here rather
// than coming from CatalogService.
export const scopeLabels: Record<ClauseScope, string> = {
  [ClauseScope.UNSPECIFIED]: "Unspecified",
  [ClauseScope.AGGREGATE]: "Total across activities",
  [ClauseScope.SINGLE]: "Any single activity",
};

export const comparatorLabels: Record<Comparator, string> = {
  [Comparator.UNSPECIFIED]: "Unspecified",
  [Comparator.GTE]: "at least",
  [Comparator.LTE]: "at most",
};

// sportSelectorLabel renders a SportSelector oneof for display, using the
// sport-group catalog to turn a group key into its human label.
export function sportSelectorLabel(
  sport: SportSelector | undefined,
  sportGroups: SportGroupInfo[],
): string {
  if (!sport || sport.selector.case === undefined) return "All sports";
  if (sport.selector.case === "group") {
    const group = sportGroups.find((g) => g.group === sport.selector.value);
    return group ? group.label : sport.selector.value;
  }
  return sport.selector.value;
}
