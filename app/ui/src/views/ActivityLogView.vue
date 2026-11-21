<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { activityClient } from "../lib/connect";
import { useCatalogStore } from "../stores/catalog";
import type { ManualActivity } from "../gen/fitter/v1/activity_pb";
import { exercises, normalizeExercise } from "../lib/exercises";
import { maskDuration, parseDuration } from "../lib/metricFormat";
import { fromDate, toDate } from "../lib/timestamp";
import { errorMessage } from "../lib/errors";

const catalog = useCatalogStore();

interface LiftRow {
  key: number;
  exercise: string;
  weight: string;
  reps: string;
}
let nextKey = 0;
const blankLift = (): LiftRow => ({ key: nextKey++, exercise: "", weight: "", reps: "" });

// datetime-local wants "YYYY-MM-DDTHH:mm" in local time.
const localNow = () => {
  const d = new Date();
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 16);
};

const form = reactive({ sport: "", name: "", when: localNow(), duration: "", distanceKm: "", lifts: [blankLift()] });
const saving = ref(false);
const error = ref("");
const showErrors = ref(false);
const saved = ref("");
const list = ref<ManualActivity[]>([]);
const listError = ref("");

const groupOf = (sportType: string) => catalog.sportGroups.find((g) => g.sportTypes.includes(sportType))?.group;
const isStrength = computed(() => groupOf(form.sport) === "strength");

const durationS = computed(() => (form.duration.trim() ? parseDuration(form.duration) : null));
const errors = computed(() => ({
  sport: form.sport ? "" : "Pick a sport.",
  when: form.when ? "" : "Pick when it happened.",
  duration: durationS.value === null ? "Enter a time like 4500 (45:00) or 11500 (1:15:00)." : "",
  distance: form.distanceKm.trim() && !(Number(form.distanceKm) >= 0) ? "Enter a distance in km." : "",
  lifts: isStrength.value
    ? form.lifts.map((l) =>
        !l.exercise.trim() && !l.weight && !l.reps
          ? ""
          : !normalizeExercise(l.exercise) || !(Number(l.weight) > 0) || !(Number(l.reps) > 0)
            ? "Exercise, weight and reps are all needed."
            : "",
      )
    : [],
}));
const hasErrors = computed(() => !!(errors.value.sport || errors.value.when || errors.value.duration || errors.value.distance || errors.value.lifts.some(Boolean)));

async function load(): Promise<void> {
  listError.value = "";
  try {
    const [resp] = await Promise.all([activityClient.listManualActivities({}), catalog.ensureLoaded()]);
    list.value = resp.activities;
  } catch (err) {
    listError.value = errorMessage(err, "Your logged activities didn't load.");
  }
}
onMounted(load);

async function save(): Promise<void> {
  showErrors.value = true;
  if (hasErrors.value) return;
  saving.value = true;
  error.value = "";
  saved.value = "";
  try {
    const resp = await activityClient.createManualActivity({
      activity: {
        sportType: form.sport,
        name: form.name.trim() || form.sport,
        startDate: fromDate(new Date(form.when)),
        utcOffsetMinutes: -new Date(form.when).getTimezoneOffset(),
        movingTimeS: BigInt(durationS.value ?? 0),
        distanceM: form.distanceKm.trim() ? Number(form.distanceKm) * 1000 : undefined,
        lifts: isStrength.value
          ? form.lifts
              .filter((l) => l.exercise.trim())
              .map((l) => ({ exercise: normalizeExercise(l.exercise), weightKg: Number(l.weight), reps: Number(l.reps) }))
          : [],
      },
    });
    if (resp.activity) list.value.unshift(resp.activity);
    saved.value = "Logged. It counts toward your goals.";
    Object.assign(form, { name: "", duration: "", distanceKm: "", lifts: [blankLift()], when: localNow() });
    showErrors.value = false;
  } catch (err) {
    error.value = errorMessage(err, "The activity wasn't saved. Try again.");
  } finally {
    saving.value = false;
  }
}

async function remove(id: bigint): Promise<void> {
  try {
    await activityClient.deleteManualActivity({ id });
    list.value = list.value.filter((a) => a.id !== id);
  } catch (err) {
    listError.value = errorMessage(err, "The activity wasn't deleted. Try again.");
  }
}

const describe = (a: ManualActivity) => {
  const when = toDate(a.startDate)?.toLocaleDateString() ?? "";
  const lifts = a.lifts.map((l) => `${l.exercise} ${l.weightKg} kg × ${l.reps}`).join(", ");
  return [a.sportType, when, lifts].filter(Boolean).join(" · ");
};
</script>

<template>
  <main class="al-page">
    <div class="al">
      <h1>Log activity</h1>
      <p class="meta">For things Strava doesn't see, like a gym session. Counts toward your goals and campaigns.</p>

      <form class="form" novalidate @submit.prevent="save">
        <div class="rb-field">
          <label for="al-sport">Sport</label>
          <select id="al-sport" v-model="form.sport" class="input" :aria-invalid="showErrors && !!errors.sport">
            <option value="" disabled>Choose…</option>
            <optgroup v-for="g in catalog.sportGroups" :key="g.group" :label="g.label">
              <option v-for="t in g.sportTypes" :key="t" :value="t">{{ t }}</option>
            </optgroup>
          </select>
          <span class="field-error" role="alert">{{ showErrors ? errors.sport : "" }}</span>
        </div>

        <div class="rb-field">
          <label for="al-name">Name <small>optional</small></label>
          <input id="al-name" v-model="form.name" class="input" maxlength="80" autocomplete="off" placeholder="Push day" />
        </div>

        <div class="rb-field">
          <label for="al-when">When</label>
          <input id="al-when" v-model="form.when" class="input" type="datetime-local" :aria-invalid="showErrors && !!errors.when" />
          <span class="field-error" role="alert">{{ showErrors ? errors.when : "" }}</span>
        </div>

        <div class="row">
          <div class="rb-field">
            <label for="al-duration">Time <small>optional</small></label>
            <input id="al-duration" class="input" :value="form.duration" inputmode="numeric" placeholder="45:00" @input="form.duration = maskDuration(($event.target as HTMLInputElement).value)" :aria-invalid="showErrors && !!errors.duration" />
            <span class="field-error" role="alert">{{ showErrors ? errors.duration : "" }}</span>
          </div>
          <div v-if="!isStrength" class="rb-field">
            <label for="al-distance">Distance (km) <small>optional</small></label>
            <input id="al-distance" v-model="form.distanceKm" class="input" inputmode="decimal" :aria-invalid="showErrors && !!errors.distance" />
            <span class="field-error" role="alert">{{ showErrors ? errors.distance : "" }}</span>
          </div>
        </div>

        <fieldset v-if="isStrength" class="rb-field lifts">
          <legend>Lifts</legend>
          <div v-for="(l, i) in form.lifts" :key="l.key" class="lift">
            <label class="visually-hidden" :for="`al-ex-${l.key}`">Lift {{ i + 1 }} exercise</label>
            <input :id="`al-ex-${l.key}`" v-model="l.exercise" class="input" list="al-exercises" placeholder="bench press" autocomplete="off" />
            <label class="visually-hidden" :for="`al-w-${l.key}`">Lift {{ i + 1 }} weight in kg</label>
            <input :id="`al-w-${l.key}`" v-model="l.weight" class="input input--num" inputmode="decimal" placeholder="kg" />
            <label class="visually-hidden" :for="`al-r-${l.key}`">Lift {{ i + 1 }} reps</label>
            <input :id="`al-r-${l.key}`" v-model="l.reps" class="input input--num" inputmode="numeric" placeholder="reps" />
            <button v-if="form.lifts.length > 1" type="button" class="pill pill--quiet" @click="form.lifts.splice(i, 1)">
              Remove<span class="visually-hidden"> lift {{ i + 1 }}</span>
            </button>
            <span class="field-error" role="alert">{{ showErrors ? errors.lifts[i] : "" }}</span>
          </div>
          <button type="button" class="pill" @click="form.lifts.push(blankLift())">Add lift</button>
          <datalist id="al-exercises"><option v-for="e in exercises" :key="e" :value="e" /></datalist>
        </fieldset>

        <p v-if="error" class="notice" role="alert">{{ error }}</p>
        <p v-if="saved" class="saved" role="status">{{ saved }}</p>
        <button type="submit" class="pill pill--primary" v-busy="saving">{{ saving ? "Saving…" : "Log activity" }}</button>
      </form>

      <section v-if="list.length || listError" class="history">
        <h2>Logged by you</h2>
        <p v-if="listError" class="field-error" role="alert">{{ listError }}</p>
        <ol>
          <li v-for="a in list" :key="String(a.id)">
            <span>
              <b>{{ a.name }}</b>
              <small>{{ describe(a) }}</small>
            </span>
            <button type="button" class="pill pill--quiet" @click="remove(a.id)">Delete<span class="visually-hidden"> {{ a.name }}</span></button>
          </li>
        </ol>
      </section>
    </div>
  </main>
</template>

<style scoped>
.al-page {
  background: var(--paper);
  min-height: calc(100vh - 58px);
}
.al {
  max-width: 560px;
  margin: 0 auto;
  padding: var(--space-5) var(--gutter) var(--space-8);
  font-family: var(--font-body);
  color: var(--ink);
}
h1 {
  margin: var(--space-3) 0 0;
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}
h2 {
  margin: var(--space-8) 0 var(--space-4);
  font: 400 1.125rem/1.1 var(--font-display);
  text-transform: uppercase;
}
.meta {
  margin: var(--space-2) 0 var(--space-6);
  color: var(--ink-muted);
  font-size: 0.875rem;
}
.form {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
}
.row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  gap: var(--space-5);
}
.lifts {
  border: 0;
  padding: 0;
  margin: 0;
}
.lift {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 5rem 4.5rem auto;
  gap: var(--space-3);
  align-items: center;
  margin-bottom: var(--space-3);
}
.lift .field-error {
  grid-column: 1 / -1;
}
.saved {
  margin: 0;
  font-weight: 700;
}
.notice {
  margin: 0;
  padding: var(--space-4) var(--space-5);
  border: 2px solid var(--ink);
  border-radius: 6px;
  background: var(--cool-blue);
}
.history ol {
  list-style: none;
  margin: 0;
  padding: 0;
}
.history li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--space-5);
  padding: var(--space-3) 0;
  border-bottom: 1px solid var(--hairline);
}
.history small {
  display: block;
  color: var(--ink-muted);
}
</style>
