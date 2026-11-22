<script setup lang="ts">
import { computed, nextTick } from "vue";
import { ExerciseMeasure } from "../gen/fitter/v1/exercise_pb";
import { blankStation, clearUnused, MAX_ROUNDS, type BlockDraft, type StationDraft } from "../lib/blocks";
import { maskDuration } from "../lib/metricFormat";
import { useExerciseStore } from "../stores/exercises";
import ExercisePicker from "./ExercisePicker.vue";

// One block of a session: its stations (exercise + result), repeated for `rounds`.
// Straight sets are one station with rounds = sets; a superset is two stations.
const props = defineProps<{ index: number; idPrefix: string; errors: Map<number, string>; showErrors: boolean; canRemove: boolean }>();
const block = defineModel<BlockDraft>({ required: true });
const emit = defineEmits<{ remove: [] }>();

const store = useExerciseStore();
const exerciseOf = (s: StationDraft) => store.byId.get(s.exerciseId);
const id = (s: string) => `${props.idPrefix}-${s}`;
const where = (i: number) => `Block ${props.index + 1}, exercise ${i + 1}`;
const headingId = computed(() => id("title"));

function step(by: number): void {
  const next = block.value.rounds + by;
  if (next >= 1 && next <= MAX_ROUNDS) block.value.rounds = next;
}

function onExercise(s: StationDraft, exerciseId: string): void {
  s.exerciseId = exerciseId;
  clearUnused(s, store.byId.get(exerciseId));
}

async function addStation(): Promise<void> {
  const s = blankStation();
  block.value.stations.push(s);
  await nextTick();
  document.getElementById(id(`ex-${s.key}`))?.focus();
}

async function removeStation(i: number): Promise<void> {
  block.value.stations.splice(i, 1);
  await nextTick();
  const near = block.value.stations[Math.max(0, i - 1)];
  (near ? document.getElementById(id(`ex-${near.key}`)) : document.getElementById(id("add")))?.focus();
}
</script>

<template>
  <section class="block" :aria-labelledby="headingId">
    <header>
      <h3 :id="headingId">Block {{ index + 1 }}</h3>
      <div class="rounds" role="group" :aria-label="`Block ${index + 1} rounds`">
        <button type="button" class="step" aria-label="Fewer rounds" :aria-disabled="block.rounds <= 1" @click="step(-1)">−</button>
        <output :for="headingId">{{ block.rounds }} {{ block.rounds === 1 ? "round" : "rounds" }}</output>
        <button type="button" class="step" aria-label="More rounds" :aria-disabled="block.rounds >= MAX_ROUNDS" @click="step(1)">+</button>
      </div>
    </header>

    <ol class="stations">
      <li v-for="(s, i) in block.stations" :key="s.key" class="station">
        <ExercisePicker
          :model-value="s.exerciseId"
          :input-id="id(`ex-${s.key}`)"
          :label="`${where(i)}`"
                    :invalid="showErrors && errors.has(s.key) && !s.exerciseId"
          :describedby="id(`st-${s.key}-error`)"
          @update:model-value="onExercise(s, $event)"
        />
        <div v-if="exerciseOf(s)" class="fields">
          <template v-if="exerciseOf(s)!.measure === ExerciseMeasure.WEIGHT_REPS">
            <label class="visually-hidden" :for="id(`w-${s.key}`)">{{ where(i) }} weight in kg</label>
            <input :id="id(`w-${s.key}`)" v-model="s.weight" class="input" inputmode="decimal" placeholder="kg" autocomplete="off" :aria-invalid="showErrors && errors.has(s.key)" :aria-describedby="id(`st-${s.key}-error`)" />
          </template>
          <template v-if="exerciseOf(s)!.measure === ExerciseMeasure.WEIGHT_REPS || exerciseOf(s)!.measure === ExerciseMeasure.REPS">
            <label class="visually-hidden" :for="id(`r-${s.key}`)">{{ where(i) }} reps</label>
            <input :id="id(`r-${s.key}`)" v-model="s.reps" class="input" inputmode="numeric" placeholder="reps" autocomplete="off" :aria-invalid="showErrors && errors.has(s.key)" :aria-describedby="id(`st-${s.key}-error`)" />
          </template>
          <template v-if="exerciseOf(s)!.measure === ExerciseMeasure.DISTANCE">
            <label class="visually-hidden" :for="id(`d-${s.key}`)">{{ where(i) }} distance in metres</label>
            <input :id="id(`d-${s.key}`)" v-model="s.distance" class="input" inputmode="decimal" placeholder="metres" autocomplete="off" :aria-invalid="showErrors && errors.has(s.key)" :aria-describedby="id(`st-${s.key}-error`)" />
          </template>
          <template v-if="exerciseOf(s)!.measure === ExerciseMeasure.TIME">
            <label class="visually-hidden" :for="id(`t-${s.key}`)">{{ where(i) }} time</label>
            <input :id="id(`t-${s.key}`)" class="input" :value="s.duration" inputmode="numeric" placeholder="1:30" autocomplete="off" :aria-invalid="showErrors && errors.has(s.key)" :aria-describedby="id(`st-${s.key}-error`)" @input="s.duration = maskDuration(($event.target as HTMLInputElement).value)" />
          </template>
          <template v-if="exerciseOf(s)!.loaded">
            <label class="visually-hidden" :for="id(`w-${s.key}`)">{{ where(i) }} weight in kg, optional</label>
            <input :id="id(`w-${s.key}`)" v-model="s.weight" class="input" inputmode="decimal" placeholder="kg (optional)" autocomplete="off" :aria-invalid="showErrors && errors.has(s.key)" :aria-describedby="id(`st-${s.key}-error`)" />
          </template>
        </div>
        <button v-if="block.stations.length > 1" type="button" class="pill pill--quiet rm" @click="removeStation(i)">
          Remove<span class="visually-hidden"> {{ where(i) }}</span>
        </button>
        <span :id="id(`st-${s.key}-error`)" class="field-error">{{ showErrors ? errors.get(s.key) : "" }}</span>
      </li>
    </ol>

    <div class="foot">
      <button :id="id('add')" type="button" class="pill" @click="addStation">Add exercise<span class="visually-hidden"> to block {{ index + 1 }}</span></button>
      <button v-if="canRemove" type="button" class="pill pill--quiet" data-remove-block @click="emit('remove')">Remove block<span class="visually-hidden"> {{ index + 1 }}</span></button>
    </div>
  </section>
</template>

<style scoped>
.block {
  padding: var(--row) var(--bib-inset);
  border: 2px solid var(--line);
  border-radius: 10px;
}
header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: var(--space-3);
}
h3 {
  margin: 0;
  font: 400 0.875rem/1.3 var(--font-display);
  text-transform: uppercase;
}
.rounds {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-weight: 700;
  font-size: 0.9375rem;
  font-variant-numeric: tabular-nums;
}
.rounds output {
  min-width: 4.75rem;
  text-align: center;
}
.step {
  width: 2.75rem;
  height: 2.75rem;
  border: 2px solid var(--line);
  border-radius: 999px;
  background: var(--paper);
  color: var(--ink);
  font: 700 1.25rem/1 var(--font-body);
  cursor: pointer;
}
.step[aria-disabled="true"] {
  border-color: var(--hairline);
  color: var(--ink-muted);
  cursor: default;
}
@media (hover: hover) {
  .step:not([aria-disabled="true"]):hover {
    background: var(--cool-blue);
  }
}
.stations {
  list-style: none;
  margin: 0;
  padding: 0;
}
.station {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-4) 0;
  border-top: 1px solid var(--hairline);
}
.station:first-child {
  border-top: 0;
  padding-top: 0;
}
.station > :first-child {
  flex: 1 1 100%;
}
.fields {
  display: flex;
  flex: 1 1 auto;
  flex-wrap: wrap;
  gap: var(--space-3);
}
.fields .input {
  flex: 1 1 5.5rem;
  min-width: 0;
  max-width: 9rem;
}
.station .field-error {
  flex: 1 1 100%;
}
.foot {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-top: var(--space-2);
}
</style>
