import type { MessageInitShape } from "@bufbuild/protobuf";
import type { Block, BlockSchema } from "../gen/fitter/v1/activity_pb";
import { ExerciseMeasure, type Exercise } from "../gen/fitter/v1/exercise_pb";
import { parseDuration } from "./metricFormat";

// A session is blocks of stations; a block repeats its stations for `rounds`. These are the
// form's drafts (strings straight from the inputs) and the functions that check them, send
// them, and describe a saved session in one line.

export interface StationDraft {
  key: number;
  exerciseId: string;
  weight: string;
  reps: string;
  distance: string; // metres
  duration: string; // masked m:ss
}

export interface BlockDraft {
  key: number;
  rounds: number;
  stations: StationDraft[];
}

export const MAX_ROUNDS = 100;

let nextKey = 0;
export const blankStation = (): StationDraft => ({ key: nextKey++, exerciseId: "", weight: "", reps: "", distance: "", duration: "" });
export const blankBlock = (): BlockDraft => ({ key: nextKey++, rounds: 1, stations: [blankStation()] });

const hasResult = (s: StationDraft) => !!(s.weight.trim() || s.reps.trim() || s.distance.trim() || s.duration.trim());
// A row nobody touched is ignored rather than reported, like a blank lift row was.
const isBlank = (s: StationDraft) => !s.exerciseId && !hasResult(s);

const pos = (v: string) => Number(v) > 0 && Number.isFinite(Number(v));
const whole = (v: string) => pos(v) && Number.isInteger(Number(v));

// Switching exercise keeps what was typed only where the new measure still uses it.
export function clearUnused(s: StationDraft, ex: Exercise | undefined): void {
  const m = ex?.measure;
  if (m !== ExerciseMeasure.WEIGHT_REPS && !(ex?.loaded)) s.weight = "";
  if (m !== ExerciseMeasure.WEIGHT_REPS && m !== ExerciseMeasure.REPS) s.reps = "";
  if (m !== ExerciseMeasure.DISTANCE) s.distance = "";
  if (m !== ExerciseMeasure.TIME) s.duration = "";
}

function stationError(s: StationDraft, ex: Exercise | undefined): string {
  if (!s.exerciseId || !ex) return "Pick an exercise from the list.";
  const weightOk = !s.weight.trim() || pos(s.weight);
  switch (ex.measure) {
    case ExerciseMeasure.WEIGHT_REPS:
      return pos(s.weight) && whole(s.reps) ? "" : "Weight and reps are both needed.";
    case ExerciseMeasure.REPS:
      return whole(s.reps) ? "" : "Reps are needed.";
    case ExerciseMeasure.DISTANCE:
      return !pos(s.distance) ? "Distance in metres is needed." : weightOk ? "" : "Weight must be above 0.";
    case ExerciseMeasure.TIME: {
      const t = s.duration.trim() ? parseDuration(s.duration) : null;
      return !t || t <= 0 ? "Time is needed, like 1:30." : weightOk ? "" : "Weight must be above 0.";
    }
    default:
      return "Pick an exercise from the list.";
  }
}

// Errors by station key. Blank rows are skipped, so a session may have no exercises at all.
export function validateBlocks(blocks: BlockDraft[], byId: Map<string, Exercise>): Map<number, string> {
  const errors = new Map<number, string>();
  for (const b of blocks) {
    for (const s of b.stations) {
      if (isBlank(s)) continue;
      const e = stationError(s, byId.get(s.exerciseId));
      if (e) errors.set(s.key, e);
    }
  }
  return errors;
}

// Blocks to send: blank rows and empty blocks dropped. Call only when validateBlocks is empty.
export function toProtoBlocks(blocks: BlockDraft[], byId: Map<string, Exercise>): MessageInitShape<typeof BlockSchema>[] {
  const out: MessageInitShape<typeof BlockSchema>[] = [];
  for (const b of blocks) {
    const stations = b.stations
      .filter((s) => !isBlank(s))
      .map((s) => {
        const m = byId.get(s.exerciseId)?.measure;
        return {
          exerciseId: s.exerciseId,
          weightKg: s.weight.trim() ? Number(s.weight) : undefined,
          reps: m === ExerciseMeasure.WEIGHT_REPS || m === ExerciseMeasure.REPS ? Number(s.reps) : undefined,
          distanceM: m === ExerciseMeasure.DISTANCE ? Number(s.distance) : undefined,
          durationS: m === ExerciseMeasure.TIME ? BigInt(parseDuration(s.duration) ?? 0) : undefined,
        };
      });
    if (stations.length) out.push({ rounds: b.rounds, stations });
  }
  return out;
}

export function formatDistance(m: number): string {
  if (m < 1000) return `${m} m`;
  const km = m / 1000;
  return `${Number.isInteger(km) ? km : km.toFixed(2).replace(/\.?0+$/, "")} km`;
}

export function formatDuration(totalS: number): string {
  const h = Math.floor(totalS / 3600);
  const m = Math.floor((totalS % 3600) / 60);
  const s = totalS % 60;
  const ss = String(s).padStart(2, "0");
  return h ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
}

type StationLike = Pick<Block["stations"][number], "exerciseName" | "weightKg" | "reps" | "distanceM" | "durationS">;

function stationText(s: StationLike): string {
  const name = s.exerciseName || "Exercise";
  const parts: string[] = [];
  if (s.distanceM !== undefined) parts.push(formatDistance(s.distanceM));
  if (s.durationS !== undefined) parts.push(formatDuration(Number(s.durationS)));
  const weight = s.weightKg !== undefined ? `${s.weightKg} kg` : "";
  if (s.reps !== undefined) {
    // "Bench press 80 kg × 5", "Pull-up × 8"
    return [name, weight, `× ${s.reps}`].filter(Boolean).join(" ");
  }
  return [name, ...parts, weight && `@ ${weight}`].filter(Boolean).join(" ");
}

// One line for a saved session, e.g. "3 × (Bench press 80 kg × 5 + Pull-up × 8); SkiErg 1 km".
export function describeBlocks(blocks: Block[]): string {
  return blocks
    .map((b) => {
      const text = b.stations.map(stationText).join(" + ");
      if (b.rounds <= 1) return text;
      return b.stations.length > 1 ? `${b.rounds} × (${text})` : `${text} · ${b.rounds} rounds`;
    })
    .join("; ");
}
