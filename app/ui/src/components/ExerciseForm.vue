<script setup lang="ts">
import { computed, ref } from "vue";
import { ExerciseMeasure, Muscle, type Exercise } from "../gen/fitter/v1/exercise_pb";
import { measureLabels, measureOrder, muscleLabels, muscleOrder } from "../lib/labels";
import { errorMessage } from "../lib/errors";
import { normalizeExercise } from "../lib/exercises";
import { useExerciseStore } from "../stores/exercises";
import { vFocus } from "../lib/focus";

// Add an exercise to the shared library without leaving the log form. It is not a <form>
// (the log form is its ancestor), so Enter in the name field submits it by hand.
const props = defineProps<{ idPrefix: string; initialName: string }>();
const emit = defineEmits<{ created: [exercise: Exercise]; cancel: [] }>();

const store = useExerciseStore();
const name = ref(props.initialName);
const measure = ref<ExerciseMeasure>(ExerciseMeasure.UNSPECIFIED);
const loaded = ref(false);
const muscles = ref<Muscle[]>([]);
const showErrors = ref(false);
const saving = ref(false);
const error = ref("");

const takesOptionalWeight = computed(() => measure.value === ExerciseMeasure.DISTANCE || measure.value === ExerciseMeasure.TIME);
const errors = computed(() => ({
  name: !normalizeExercise(name.value) ? "Name the exercise." : name.value.trim().length > 60 ? "Keep the name to 60 characters." : "",
  measure: measure.value === ExerciseMeasure.UNSPECIFIED ? "Pick how it's measured." : "",
  muscles: muscles.value.length ? "" : "Pick at least one muscle.",
}));
const id = (s: string) => `${props.idPrefix}-${s}`;

function toggleMuscle(m: Muscle): void {
  muscles.value = muscles.value.includes(m) ? muscles.value.filter((x) => x !== m) : [...muscles.value, m];
}

async function submit(): Promise<void> {
  showErrors.value = true;
  if (errors.value.name || errors.value.measure || errors.value.muscles) return;
  saving.value = true;
  error.value = "";
  try {
    emit("created", await store.create({ name: name.value, measure: measure.value, loaded: takesOptionalWeight.value && loaded.value, muscles: muscles.value }));
  } catch (err) {
    error.value = errorMessage(err, "The exercise wasn't added. Try again.");
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div class="xf" role="group" :aria-labelledby="id('title')" @keydown.esc.stop="emit('cancel')">
    <h4 :id="id('title')">New exercise</h4>

    <div class="rb-field">
      <label :for="id('name')">Name</label>
      <input
        :id="id('name')"
        v-model="name"
        v-focus="true"
        class="input"
        maxlength="60"
        autocomplete="off"
        :aria-invalid="showErrors && !!errors.name"
        :aria-describedby="id('name-error')"
        @keydown.enter.prevent="submit"
      />
      <span :id="id('name-error')" class="field-error">{{ showErrors ? errors.name : "" }}</span>
    </div>

    <fieldset class="rb-field">
      <legend>How is it measured?</legend>
      <div class="seg" role="radiogroup" :aria-describedby="id('measure-error')">
        <label v-for="m in measureOrder" :key="m" class="choice choice--pill">
          <input v-model="measure" type="radio" :name="id('measure')" :value="m" />
          <span>{{ measureLabels[m] }}</span>
        </label>
      </div>
      <label v-if="takesOptionalWeight" class="choice choice--pill opt">
        <input v-model="loaded" type="checkbox" />
        <span>A weight is optional</span>
      </label>
      <span :id="id('measure-error')" class="field-error">{{ showErrors ? errors.measure : "" }}</span>
    </fieldset>

    <fieldset class="rb-field">
      <legend>Muscles it works</legend>
      <div class="seg" :aria-describedby="id('muscles-error')">
        <label v-for="m in muscleOrder" :key="m" class="choice choice--pill">
          <input type="checkbox" :checked="muscles.includes(m)" @change="toggleMuscle(m)" />
          <span>{{ muscleLabels[m] }}</span>
        </label>
      </div>
      <span :id="id('muscles-error')" class="field-error">{{ showErrors ? errors.muscles : "" }}</span>
    </fieldset>

    <p v-if="error" class="field-error" role="alert">{{ error }}</p>
    <div class="actions">
      <button type="button" class="pill pill--primary" v-busy="saving" @click="submit">{{ saving ? "Adding…" : "Add exercise" }}</button>
      <button type="button" class="pill pill--quiet" @click="emit('cancel')">Cancel</button>
    </div>
  </div>
</template>

<style scoped>
.xf {
  margin-top: var(--space-3);
  padding: var(--row) var(--bib-inset);
  border: 2px solid var(--line);
  border-radius: 10px;
  background: var(--paper);
}
h4 {
  margin: 0 0 var(--space-4);
  font: 400 0.875rem/1.3 var(--font-display);
  text-transform: uppercase;
}
fieldset {
  border: 0;
  padding: 0;
  margin: 0 0 var(--space-5);
  min-width: 0;
}
.seg {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.opt {
  align-self: flex-start;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
</style>
