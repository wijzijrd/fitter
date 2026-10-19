import type { Campaign } from "../gen/fitter/v1/campaign_pb";
import { toDate } from "./timestamp";

export const DAY = 86_400_000;

export type Phase = "upcoming" | "live" | "finished";

export function campaignPhase(c: Campaign, now = Date.now()): Phase {
  const start = toDate(c.startDate)?.getTime() ?? 0;
  const end = toDate(c.endDate)?.getTime() ?? 0;
  if (now < start) return "upcoming";
  return now > end ? "finished" : "live";
}

// daysUntil rounds up, so anything later today still reads as "1 day".
export function daysUntil(d: Date | null, now = Date.now()): number {
  return d ? Math.max(0, Math.ceil((d.getTime() - now) / DAY)) : 0;
}

export function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? "" : "s"}`;
}

const short = new Intl.DateTimeFormat(undefined, { day: "numeric", month: "short" });

// Non-breaking, so "18 Oct" never splits across lines.
export function shortDate(d: Date | null): string {
  return d ? short.format(d).replace(/ /g, "\u00a0") : "";
}

export function dateRange(start: Date | null, end: Date | null): string {
  return start && end ? `${shortDate(start)} – ${shortDate(end)}` : "";
}
