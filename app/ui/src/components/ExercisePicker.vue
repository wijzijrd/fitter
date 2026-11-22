<script setup lang="ts">
import { computed, nextTick, ref, useTemplateRef, watch } from "vue";
import { ExerciseMeasure, type Exercise } from "../gen/fitter/v1/exercise_pb";
import { measureLabels, muscleLabels } from "../lib/labels";
import { normalizeExercise } from "../lib/exercises";
import { useExerciseStore } from "../stores/exercises";
import ExerciseForm from "./ExerciseForm.vue";

// An editable combobox with list autocomplete (WAI-ARIA 1.2): type to filter by name or
// muscle, arrows and Enter to choose, Escape to close. The last option adds a new exercise
// inline. The model is the exercise id; it is "" until one is chosen.
const props = defineProps<{ inputId: string; label: string; invalid?: boolean; describedby?: string }>();
const model = defineModel<string>({ default: "" });

const store = useExerciseStore();
const input = useTemplateRef<HTMLInputElement>("input");
const wrap = useTemplateRef<HTMLElement>("wrap");

const selected = computed(() => store.byId.get(model.value));
const query = ref(selected.value?.name ?? "");
const open = ref(false);
const active = ref(-1);
const adding = ref(false);

// While the box just shows the chosen name, offer the whole list rather than only that name.
const filterText = computed(() => (selected.value && query.value === selected.value.name ? "" : normalizeExercise(query.value)));
const matches = computed(() => {
  const words = filterText.value.split(" ").filter(Boolean);
  if (!words.length) return store.available;
  return store.available.filter((e) => {
    const hay = `${e.key} ${e.muscles.map((m) => muscleLabels[m].toLowerCase()).join(" ")}`;
    return words.every((w) => hay.includes(w));
  });
});
// Typing an archived name offers "Add", which the server answers by restoring it.
const canAdd = computed(() => !!filterText.value && !store.available.some((e) => e.key === filterText.value));
const optionCount = computed(() => matches.value.length + (canAdd.value ? 1 : 0));

const listId = computed(() => `${props.inputId}-list`);
const optId = (i: number) => `${props.inputId}-opt-${i}`;
const activeId = computed(() => (open.value && active.value >= 0 ? optId(active.value) : undefined));

function meta(e: Exercise): string {
  const how = e.measure === ExerciseMeasure.WEIGHT_REPS ? measureLabels[e.measure] : e.loaded ? `${measureLabels[e.measure]} (weight optional)` : measureLabels[e.measure];
  const m = e.muscles.map((x) => muscleLabels[x]).join(", ");
  return m ? `${how} · ${m}` : how;
}

function show(): void {
  open.value = true;
  const at = selected.value ? matches.value.findIndex((e) => e.id === selected.value!.id) : -1;
  active.value = at >= 0 ? at : optionCount.value ? 0 : -1;
}
function close(): void {
  open.value = false;
  active.value = -1;
}
function pick(e: Exercise): void {
  model.value = e.id;
  query.value = e.name;
  close();
}
function choose(i: number): void {
  if (i < matches.value.length) pick(matches.value[i]);
  else if (canAdd.value) {
    adding.value = true;
    close();
  }
}

function onInput(ev: Event): void {
  query.value = (ev.target as HTMLInputElement).value;
  model.value = "";
  open.value = true;
  active.value = optionCount.value ? 0 : -1;
}

function onKeydown(ev: KeyboardEvent): void {
  if (ev.key === "ArrowDown") {
    ev.preventDefault();
    if (!open.value) show();
    else active.value = Math.min(active.value + 1, optionCount.value - 1);
  } else if (ev.key === "ArrowUp") {
    ev.preventDefault();
    if (open.value) active.value = Math.max(active.value - 1, 0);
  } else if (ev.key === "Enter" && open.value && active.value >= 0) {
    ev.preventDefault(); // choose, don't submit the log form
    choose(active.value);
  } else if (ev.key === "Escape" && open.value) {
    ev.stopPropagation();
    close();
  }
}

// Leaving the control: close, take an exact name typed in full, and put the chosen name back.
function onFocusOut(ev: FocusEvent): void {
  if (ev.relatedTarget instanceof Node && wrap.value?.contains(ev.relatedTarget)) return;
  close();
  if (!model.value) {
    const exact = store.available.find((e) => e.key === normalizeExercise(query.value));
    if (exact) pick(exact);
  } else if (selected.value) {
    query.value = selected.value.name;
  }
}

async function onCreated(e: Exercise): Promise<void> {
  adding.value = false;
  pick(e);
  await nextTick();
  input.value?.focus();
}
async function cancelAdd(): Promise<void> {
  adding.value = false;
  await nextTick();
  input.value?.focus();
}

watch(active, async (i) => {
  if (i < 0) return;
  await nextTick();
  document.getElementById(optId(i))?.scrollIntoView({ block: "nearest" });
});
watch(model, (v) => {
  if (v && selected.value) query.value = selected.value.name;
});
</script>

<template>
  <div ref="wrap" class="pick" @focusout="onFocusOut">
    <div class="box">
      <label :for="inputId" class="visually-hidden">{{ label }}</label>
      <input
        :id="inputId"
        ref="input"
        class="input"
        role="combobox"
        aria-autocomplete="list"
        :aria-expanded="open"
        :aria-controls="listId"
        :aria-activedescendant="activeId"
        :aria-invalid="invalid"
        :aria-describedby="describedby"
        autocomplete="off"
        autocapitalize="off"
        placeholder="Choose an exercise"
        :value="query"
        @input="onInput"
        @keydown="onKeydown"
        @focus="show"
        @click="open || show()"
      />
      <ul v-show="open" :id="listId" class="list" role="listbox" :aria-label="label">
        <li
          v-for="(e, i) in matches"
          :id="optId(i)"
          :key="e.id"
          role="option"
          :aria-selected="i === active"
          :class="{ active: i === active, chosen: e.id === model }"
          @mousedown.prevent
          @mousemove="active = i"
          @click="choose(i)"
        >
          <span class="opt-name">{{ e.name }}</span>
          <span class="opt-meta">{{ meta(e) }}</span>
        </li>
        <li
          v-if="canAdd"
          :id="optId(matches.length)"
          role="option"
          class="add"
          :aria-selected="active === matches.length"
          :class="{ active: active === matches.length }"
          @mousedown.prevent
          @mousemove="active = matches.length"
          @click="choose(matches.length)"
        >
          <span class="opt-name">Add “{{ query.trim() }}” as a new exercise</span>
        </li>
        <li v-if="!optionCount" role="presentation" class="none">No exercises yet. Type a name to add one.</li>
      </ul>
    </div>
    <ExerciseForm v-if="adding" :id-prefix="`${inputId}-new`" :initial-name="query.trim()" @created="onCreated" @cancel="cancelAdd" />
  </div>
</template>

<style scoped>
.pick {
  min-width: 0;
}
.box {
  position: relative;
}
.list {
  position: absolute;
  inset: calc(100% + var(--space-1)) 0 auto 0;
  z-index: 20;
  max-height: 16rem;
  margin: 0;
  padding: var(--space-1) 0;
  overflow-y: auto;
  list-style: none;
  border: 2px solid var(--line);
  border-radius: 10px;
  background: var(--paper);
}
.list li {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-height: 2.75rem;
  justify-content: center;
  padding: var(--space-2) var(--space-4);
  cursor: pointer;
}
.list li.active {
  background: var(--cool-blue);
}
.list li.chosen .opt-name::after {
  content: " ✓";
}
.list li.none {
  cursor: default;
  color: var(--ink-muted);
  font-size: 0.875rem;
}
.opt-name {
  font-weight: 700;
  font-size: 0.9375rem;
}
.add .opt-name {
  font-weight: 600;
}
.opt-meta {
  color: var(--ink-muted);
  font-size: 0.8125rem;
}
</style>
