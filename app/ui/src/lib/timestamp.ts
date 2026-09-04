import { timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

export function toDate(ts: Timestamp | undefined): Date | null {
  return ts ? timestampDate(ts) : null;
}

export function fromDate(d: Date): Timestamp {
  return timestampFromDate(d);
}
