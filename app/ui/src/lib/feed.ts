import type { Block } from "../gen/fitter/v1/activity_pb";
import type { FeedActivity } from "../gen/fitter/v1/feed_pb";
import type { FeedSettings } from "../gen/fitter/v1/user_pb";
import { describeBlock, formatDuration } from "./blocks";
import { DAY } from "./schedule";
import { toDate } from "./timestamp";

// The Feed tab's rules, kept out of the view so they can be tested: which rows show, how they
// group by day, what is new since the caller last looked, and the detail line under each row.

export const FEED_DAYS = 7;
export const MAX_LIFT_LINES = 3;

// Everyone's settings default to on until the server says otherwise.
export const allDetailsOn: FeedSettings = { showPace: true, showPower: true, showEffort: true, showLifts: true } as FeedSettings;

const startOf = (a: FeedActivity) => toDate(a.startDate)?.getTime() ?? 0;

// The server sends the campaign's latest 100 activities (the bibs' "last active" notes read the
// same list), so the one-week window is applied here.
export function recent(rows: FeedActivity[], now: number): FeedActivity[] {
  return rows.filter((a) => startOf(a) >= now - FEED_DAYS * DAY);
}

// shortcut: "new" compares the activity's start time with the last look, not when it synced, so
// a workout that syncs after the caller looked but started before it won't be marked new.
export const isNew = (a: FeedActivity, seenAt: Date | undefined, meId: string | undefined): boolean =>
  !!seenAt && a.userId !== meId && startOf(a) > seenAt.getTime();

const dayKey = (d: Date) => `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;

export function dayLabel(d: Date, now: Date): string {
  if (dayKey(d) === dayKey(now)) return "Today";
  if (dayKey(d) === dayKey(new Date(now.getTime() - DAY))) return "Yesterday";
  return d.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" });
}

export interface DayGroup {
  label: string;
  rows: FeedActivity[];
}

// Rows arrive newest first and stay so; each local day gets one heading.
export function groupByDay(rows: FeedActivity[], now: Date): DayGroup[] {
  const groups: DayGroup[] = [];
  let key = "";
  for (const a of rows) {
    const d = toDate(a.startDate) ?? new Date(0);
    if (dayKey(d) !== key) {
      key = dayKey(d);
      groups.push({ label: dayLabel(d, now), rows: [] });
    }
    groups[groups.length - 1].rows.push(a);
  }
  return groups;
}

// Pace for runs and walks, speed for rides, nothing for anything else (or without a distance).
export function paceOrSpeed(a: FeedActivity): string {
  const t = Number(a.movingTimeS);
  if (!(a.distanceM >= 100) || !(t > 0)) return "";
  if (a.sportGroup === "run" || a.sportGroup === "walk") return `${formatDuration(Math.round(t / (a.distanceM / 1000)))} /km`;
  if (a.sportGroup === "ride") return `${((a.distanceM / 1000) / (t / 3600)).toFixed(1)} km/h`;
  return "";
}

// The Meta line under a row, from what the viewer chose to see.
export function detailParts(a: FeedActivity, s: FeedSettings): string[] {
  const out: string[] = [];
  if (s.showPace) out.push(paceOrSpeed(a));
  if (s.showPower && a.averageWatts !== undefined && a.averageWatts > 0) out.push(`${Math.round(a.averageWatts)} W`);
  if (s.showEffort && a.sufferScore !== undefined && a.sufferScore > 0) out.push(`Effort ${Math.round(a.sufferScore)}`);
  return out.filter(Boolean);
}

// A manual session's blocks, one line each, cut to three with a count of the rest.
export function liftLines(blocks: Block[]): string[] {
  const lines = blocks.map(describeBlock).filter(Boolean);
  if (lines.length <= MAX_LIFT_LINES) return lines;
  return [...lines.slice(0, MAX_LIFT_LINES), `+${lines.length - MAX_LIFT_LINES} more`];
}

function names(list: string[]): string {
  if (list.length <= 1) return list[0] ?? "";
  if (list.length === 2) return `${list[0]} and ${list[1]}`;
  if (list.length === 3) return `${list[0]}, ${list[1]} and ${list[2]}`;
  return `${list[0]}, ${list[1]} and ${list.length - 2} others`;
}

// "Sam, Jo and 2 others logged 6 activities since you last looked." (empty when nothing is new)
export function pulse(newRows: FeedActivity[]): string {
  if (!newRows.length) return "";
  const who = [...new Set(newRows.map((a) => a.userName))];
  return `${names(who)} logged ${newRows.length} ${newRows.length === 1 ? "activity" : "activities"} since you last looked.`;
}
