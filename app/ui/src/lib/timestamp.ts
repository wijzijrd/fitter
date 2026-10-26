import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

export function toDate(ts: Timestamp | undefined): Date | null {
  return ts ? timestampDate(ts) : null;
}

export function fromDate(d: Date): Timestamp {
  return timestampFromDate(d);
}

const relative = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });

// formatRelative renders "2 hours ago" / "yesterday" style text for recent dates;
// compact ("2h ago") is for tight spots, hand-built because Intl's narrow style
// still prints "hr" in some engines.
export function formatRelative(d: Date, compact = false, now = new Date()): string {
  const minutes = Math.round((d.getTime() - now.getTime()) / 60000);
  const hours = Math.round(minutes / 60);
  const days = Math.round(hours / 24);
  if (compact) {
    const [n, unit] = Math.abs(minutes) < 60 ? [minutes, "m"] : Math.abs(hours) < 24 ? [hours, "h"] : [days, "d"];
    return `${Math.abs(n)}${unit} ago`;
  }
  if (Math.abs(minutes) < 60) return relative.format(minutes, "minute");
  if (Math.abs(hours) < 24) return relative.format(hours, "hour");
  return relative.format(days, "day");
}
