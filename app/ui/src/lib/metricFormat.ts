import { ActivityMetric, MetricUnit } from "../gen/fitter/v1/common_pb";

// This module derives all metric-value formatting/parsing from a fetched
// MetricUnit rather than a hand-written switch per metric (see
// stores/catalog.ts) — 17 metrics is too many for a per-metric switch to
// stay maintained by hand, and the unit is the only thing formatting
// actually depends on.

function pad2(n: number): string {
  return n.toString().padStart(2, "0");
}

// formatSeconds renders a base "seconds" value as "H:MM:SS" once it's an
// hour or more, otherwise "M:SS" — matching how people actually read
// workout durations and paces.
function formatSeconds(totalSeconds: number): string {
  const s = Math.max(0, Math.round(totalSeconds));
  const hours = Math.floor(s / 3600);
  const minutes = Math.floor((s % 3600) / 60);
  const secs = s % 60;
  if (hours > 0) return `${hours}:${pad2(minutes)}:${pad2(secs)}`;
  return `${minutes}:${pad2(secs)}`;
}

// parseDuration parses "mm:ss" or "h:mm:ss" (also tolerates a bare number of
// seconds) back into a base seconds value. Returns null if unparsable.
export function parseDuration(input: string): number | null {
  const trimmed = input.trim();
  if (trimmed === "") return null;
  if (/^\d+$/.test(trimmed)) return Number(trimmed);
  const parts = trimmed.split(":").map((p) => p.trim());
  if (parts.length < 2 || parts.length > 3 || parts.some((p) => p === "" || Number.isNaN(Number(p)))) {
    return null;
  }
  const nums = parts.map(Number);
  let seconds = 0;
  if (nums.length === 3) {
    const [h, m, s] = nums;
    seconds = h * 3600 + m * 60 + s;
  } else {
    const [m, s] = nums;
    seconds = m * 60 + s;
  }
  return seconds;
}

// isDurationUnit reports whether a metric's base value is a count of
// seconds that should be edited as mm:ss rather than a plain number.
export function isDurationUnit(unit: MetricUnit): boolean {
  return unit === MetricUnit.SECONDS || unit === MetricUnit.SECONDS_PER_KM;
}

// isDistanceUnit reports whether a metric's base value is meters and should
// be edited in km.
export function isDistanceUnit(unit: MetricUnit): boolean {
  return unit === MetricUnit.METERS;
}

// unitSuffix is the short label shown next to a plain-number input/display
// for units that don't get a friendlier conversion.
export function unitSuffix(unit: MetricUnit): string {
  switch (unit) {
    case MetricUnit.BPM:
      return "bpm";
    case MetricUnit.WATTS:
      return "W";
    case MetricUnit.KILOJOULES:
      return "kJ";
    case MetricUnit.KILOCALORIES:
      return "kcal";
    case MetricUnit.RPM:
      return "rpm";
    case MetricUnit.COUNT:
      return "";
    case MetricUnit.SCORE:
      return "score";
    case MetricUnit.METERS_PER_SECOND:
      return "km/h";
    default:
      return "";
  }
}

const whole = new Intl.NumberFormat("en", { maximumFractionDigits: 0 });
const tenth = new Intl.NumberFormat("en", { maximumFractionDigits: 1 });

// formatMetricValue renders a raw base-unit value (meters, seconds, a plain
// count, ...) the way a person should read it — used for progress displays,
// not for editing. Trailing zeros drop: "536 km", "62.3 km", "2h", "12h 30m".
// metric refines the few cases the unit alone can't: elevation stays in
// metres, and activity counts read as "activities".
export function formatMetricValue(unit: MetricUnit, value: number, metric?: ActivityMetric): string {
  if (metric === ActivityMetric.ELEVATION_GAIN) return `${whole.format(value)} m`;
  if (metric === ActivityMetric.ACTIVITY_COUNT) {
    const n = Math.round(value);
    return `${whole.format(n)} ${n === 1 ? "activity" : "activities"}`;
  }
  switch (unit) {
    case MetricUnit.METERS:
      return value > 0 && value < 1000 ? `${whole.format(value)} m` : `${tenth.format(Math.round(value / 100) / 10)} km`;
    case MetricUnit.SECONDS: {
      if (value < 3600) return formatSeconds(value);
      const minutes = Math.round(value / 60);
      const h = Math.floor(minutes / 60);
      const m = minutes % 60;
      return m ? `${h}h ${m}m` : `${h}h`;
    }
    case MetricUnit.SECONDS_PER_KM:
      return `${formatSeconds(value)}/km`;
    case MetricUnit.METERS_PER_SECOND:
      return `${tenth.format(value * 3.6)} km/h`;
    case MetricUnit.COUNT:
    case MetricUnit.SCORE:
      return whole.format(value);
    default: {
      const suffix = unitSuffix(unit);
      return suffix ? `${whole.format(value)} ${suffix}` : whole.format(value);
    }
  }
}

// baseValueToEditString converts a base-unit wire value into the string an
// <Input> should show for editing (km for distance, mm:ss for durations,
// otherwise the plain number).
export function baseValueToEditString(unit: MetricUnit, value: number): string {
  if (isDistanceUnit(unit)) {
    const km = value / 1000;
    return Number.isFinite(km) ? String(Math.round(km * 1000) / 1000) : "";
  }
  if (isDurationUnit(unit)) {
    return formatSeconds(value);
  }
  if (unit === MetricUnit.METERS_PER_SECOND) {
    const kmh = value * 3.6;
    return Number.isFinite(kmh) ? String(Math.round(kmh * 1000) / 1000) : "";
  }
  return Number.isFinite(value) ? String(value) : "";
}

// editStringToBaseValue parses the string a user typed into an <Input> back
// into the base unit the wire format expects. Returns null when the input
// can't be parsed, so callers can surface a validation error instead of
// silently sending NaN/0.
export function editStringToBaseValue(unit: MetricUnit, input: string): number | null {
  const trimmed = input.trim();
  if (trimmed === "") return null;
  if (isDistanceUnit(unit)) {
    const km = Number(trimmed);
    if (Number.isNaN(km)) return null;
    return km * 1000;
  }
  if (isDurationUnit(unit)) {
    return parseDuration(trimmed);
  }
  if (unit === MetricUnit.METERS_PER_SECOND) {
    const kmh = Number(trimmed);
    if (Number.isNaN(kmh)) return null;
    return kmh / 3.6;
  }
  const n = Number(trimmed);
  return Number.isNaN(n) ? null : n;
}

// editPlaceholder returns a hint like "km", "mm:ss", or a unit suffix for
// use as an input placeholder/adornment.
export function editUnitHint(unit: MetricUnit): string {
  if (isDistanceUnit(unit)) return "km";
  if (isDurationUnit(unit)) return "mm:ss";
  return unitSuffix(unit);
}
