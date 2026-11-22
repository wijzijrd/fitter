import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { exerciseClient } from "../lib/connect";
import { ExerciseMeasure, type Exercise, type Muscle } from "../gen/fitter/v1/exercise_pb";

// The shared exercise library, fetched once and kept in memory. Adding one from the log form
// goes through here so the new exercise is selectable straight away.
export const useExerciseStore = defineStore("exercises", () => {
  const exercises = ref<Exercise[]>([]);
  const loaded = ref(false);
  let inFlight: Promise<void> | null = null;

  const byId = computed(() => new Map(exercises.value.map((e) => [e.id, e])));
  // What new logs and goals may use. Archived ones stay in byId so history still resolves them.
  const available = computed(() => exercises.value.filter((e) => !e.archived));
  // Exercises a max-weight goal can track: they record a weight.
  const weightBearing = computed(() => available.value.filter((e) => e.measure === ExerciseMeasure.WEIGHT_REPS || e.loaded));

  async function ensureLoaded(): Promise<void> {
    if (loaded.value) return;
    if (inFlight) return inFlight;
    inFlight = (async () => {
      try {
        exercises.value = (await exerciseClient.listExercises({})).exercises;
        loaded.value = true;
      } finally {
        inFlight = null;
      }
    })();
    return inFlight;
  }

  // Keeps the list ordered by name, replacing an exercise already present.
  function upsert(e: Exercise): void {
    const rest = exercises.value.filter((x) => x.id !== e.id);
    exercises.value = [...rest, e].sort((a, b) => a.name.localeCompare(b.name));
  }

  // The server returns the stored exercise when the name already exists (restoring it if it was
  // archived); callers just select it.
  async function create(input: { name: string; measure: ExerciseMeasure; loaded: boolean; muscles: Muscle[] }): Promise<Exercise> {
    const resp = await exerciseClient.createExercise(input);
    upsert(resp.exercise!);
    return resp.exercise!;
  }

  return { exercises, loaded, byId, available, weightBearing, ensureLoaded, upsert, create };
});
