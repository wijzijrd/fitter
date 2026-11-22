import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { BlockSchema, StationSchema } from "../gen/fitter/v1/activity_pb";
import { ExerciseMeasure, ExerciseSchema, Muscle, type Exercise } from "../gen/fitter/v1/exercise_pb";
import { blankBlock, blankStation, clearUnused, describeBlocks, formatDistance, formatDuration, toProtoBlocks, validateBlocks } from "./blocks";

const ex = (id: string, measure: ExerciseMeasure, loaded = false): Exercise =>
  create(ExerciseSchema, { id, name: id, key: id, measure, loaded, muscles: [Muscle.CORE] });
const library = new Map([
  ["bench", ex("bench", ExerciseMeasure.WEIGHT_REPS)],
  ["pullup", ex("pullup", ExerciseMeasure.REPS)],
  ["ski", ex("ski", ExerciseMeasure.DISTANCE)],
  ["sled", ex("sled", ExerciseMeasure.DISTANCE, true)],
  ["plank", ex("plank", ExerciseMeasure.TIME)],
  ["carry", ex("carry", ExerciseMeasure.TIME, true)],
]);
const st = (exerciseId: string, f: Partial<ReturnType<typeof blankStation>> = {}) => ({ ...blankStation(), exerciseId, ...f });
const blockOf = (...stations: ReturnType<typeof blankStation>[]) => ({ ...blankBlock(), stations });

describe("validateBlocks", () => {
  it("ignores blank rows, so an empty session is fine", () => {
    expect(validateBlocks([blankBlock(), blankBlock()], library).size).toBe(0);
  });

  it("asks for the fields each measure needs", () => {
    const rows = [
      st("bench", { weight: "80" }), // no reps
      st("bench", { weight: "80", reps: "5.5" }), // reps must be whole
      st("pullup"),
      st("ski"),
      st("plank", { duration: "0:00" }),
      st("sled", { distance: "50", weight: "-5" }), // weight optional but must be > 0 if given
    ];
    const errors = validateBlocks([blockOf(...rows)], library);
    expect(rows.every((r) => errors.has(r.key))).toBe(true);
  });

  it("accepts complete rows, with optional weight only where loaded", () => {
    const rows = [
      st("bench", { weight: "80", reps: "5" }),
      st("pullup", { reps: "8" }),
      st("ski", { distance: "1000" }),
      st("sled", { distance: "50", weight: "100" }),
      st("plank", { duration: "1:30" }),
      st("carry", { duration: "1:00", weight: "20" }),
    ];
    expect(validateBlocks([blockOf(...rows)], library).size).toBe(0);
  });

  it("flags a value typed with no exercise, and an unknown exercise id", () => {
    const a = st("", { reps: "5" });
    const b = st("gone", { reps: "5" });
    const errors = validateBlocks([blockOf(a, b)], library);
    expect(errors.has(a.key) && errors.has(b.key)).toBe(true);
  });
});

describe("toProtoBlocks", () => {
  it("drops blank rows and empty blocks and sends only the measure's fields", () => {
    const b1 = { ...blockOf(st("bench", { weight: "80", reps: "5" }), st("")), rounds: 3 };
    const b2 = blockOf(st("plank", { duration: "1:30", reps: "9" })); // stale reps are not sent
    const out = toProtoBlocks([blankBlock(), b1, b2], library);
    expect(out).toEqual([
      { rounds: 3, stations: [{ exerciseId: "bench", weightKg: 80, reps: 5, distanceM: undefined, durationS: undefined }] },
      { rounds: 1, stations: [{ exerciseId: "plank", weightKg: undefined, reps: undefined, distanceM: undefined, durationS: 90n }] },
    ]);
  });
});

describe("clearUnused", () => {
  it("keeps weight only for weight × reps or a loaded exercise", () => {
    const s = st("x", { weight: "5", reps: "5", distance: "10", duration: "1:00" });
    clearUnused(s, library.get("plank"));
    expect(s).toMatchObject({ weight: "", reps: "", distance: "", duration: "1:00" });
    const t = st("x", { weight: "5", distance: "10" });
    clearUnused(t, library.get("sled"));
    expect(t).toMatchObject({ weight: "5", distance: "10" });
  });
});

describe("describeBlocks", () => {
  const station = (init: object) => create(StationSchema, init);
  it("writes straight sets, supersets and stations on one line", () => {
    const blocks = [
      create(BlockSchema, { rounds: 3, stations: [station({ exerciseName: "Bench press", weightKg: 80, reps: 5 }), station({ exerciseName: "Pull-up", reps: 8 })] }),
      create(BlockSchema, { rounds: 4, stations: [station({ exerciseName: "Squat", weightKg: 100, reps: 5 })] }),
      create(BlockSchema, { rounds: 1, stations: [station({ exerciseName: "SkiErg", distanceM: 1000 })] }),
      create(BlockSchema, { rounds: 1, stations: [station({ exerciseName: "Sled push", distanceM: 50, weightKg: 100 }), station({ exerciseName: "Plank", durationS: 90n })] }),
    ];
    expect(describeBlocks(blocks)).toBe(
      "3 × (Bench press 80 kg × 5 + Pull-up × 8); Squat 100 kg × 5 · 4 rounds; SkiErg 1 km; Sled push 50 m @ 100 kg + Plank 1:30",
    );
  });
});

describe("formatting", () => {
  it("formats distances and durations", () => {
    expect(formatDistance(50)).toBe("50 m");
    expect(formatDistance(1500)).toBe("1.5 km");
    expect(formatDistance(1000)).toBe("1 km");
    expect(formatDuration(90)).toBe("1:30");
    expect(formatDuration(3725)).toBe("1:02:05");
  });
});
