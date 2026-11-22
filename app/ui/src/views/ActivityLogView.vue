<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from "vue";
import { activityClient } from "../lib/connect";
import { useCatalogStore } from "../stores/catalog";
import type { ManualActivity } from "../gen/fitter/v1/activity_pb";
import { useExerciseStore } from "../stores/exercises";
import { blankBlock, describeBlocks, toProtoBlocks, validateBlocks, type BlockDraft } from "../lib/blocks";
import BlockEditor from "../components/BlockEditor.vue";
import { focusNthOr } from "../lib/focus";
import { maskDuration, parseDuration } from "../lib/metricFormat";
import { fromDate, toDate } from "../lib/timestamp";
import { errorMessage } from "../lib/errors";

const catalog = useCatalogStore();
const exerciseStore = useExerciseStore();

// datetime-local wants "YYYY-MM-DDTHH:mm" in local time.
const localNow = () => {
  const d = new Date();
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 16);
};

const form = reactive({ sport: "", name: "", when: localNow(), duration: "", distanceKm: "", blocks: [blankBlock()] as BlockDraft[] });
const saving = ref(false);
const error = ref("");
const showErrors = ref(false);
const saved = ref("");
const list = ref<ManualActivity[]>([]);
const listError = ref("");
const confirmDelete = ref<bigint | null>(null);
const deleting = ref(false);
const kept = ref<bigint | null>(null);

const groupOf = (sportType: string) => catalog.sportGroups.find((g) => g.sportTypes.includes(sportType))?.group;
// Lifting and HIIT / circuit sessions are logged as blocks of exercises.
const hasBlocks = computed(() => ["strength", "circuits"].includes(groupOf(form.sport) ?? ""));

const durationS = computed(() => (form.duration.trim() ? parseDuration(form.duration) : null));
const errors = computed(() => ({
  sport: form.sport ? "" : "Pick a sport.",
  when: form.when ? "" : "Pick when it happened.",
  duration: form.duration.trim() && durationS.value === null ? "Enter a time like 4500 (45:00) or 11500 (1:15:00)." : "",
  distance: form.distanceKm.trim() && !(Number(form.distanceKm) >= 0) ? "Enter a distance in km." : "",
  blocks: hasBlocks.value ? validateBlocks(form.blocks, exerciseStore.byId) : new Map<number, string>(),
}));
const hasErrors = computed(() => !!(errors.value.sport || errors.value.when || errors.value.duration || errors.value.distance || errors.value.blocks.size));

async function load(): Promise<void> {
  listError.value = "";
  try {
    const [resp] = await Promise.all([activityClient.listManualActivities({}), catalog.ensureLoaded(), exerciseStore.ensureLoaded()]);
    list.value = resp.activities;
  } catch (err) {
    listError.value = errorMessage(err, "Your logged activities didn't load.");
  }
}
onMounted(load);

async function save(): Promise<void> {
  showErrors.value = true;
  if (hasErrors.value) {
    // Errors are linked to their fields, so landing on the first one reads its message.
    await nextTick();
    document.querySelector<HTMLElement>('.form [aria-invalid="true"]')?.focus();
    return;
  }
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
        blocks: hasBlocks.value ? toProtoBlocks(form.blocks, exerciseStore.byId) : [],
      },
    });
    if (resp.activity) list.value.unshift(resp.activity);
    saved.value = "Logged. It counts toward your goals.";
    Object.assign(form, { name: "", duration: "", distanceKm: "", blocks: [blankBlock()], when: localNow() });
    showErrors.value = false;
  } catch (err) {
    error.value = errorMessage(err, "The activity wasn't saved. Try again.");
  } finally {
    saving.value = false;
  }
}

async function remove(id: bigint, i: number): Promise<void> {
  deleting.value = true;
  listError.value = "";
  try {
    await activityClient.deleteManualActivity({ id });
    list.value = list.value.filter((a) => a.id !== id);
    confirmDelete.value = null;
    focusNthOr("[data-delete]", i, list.value.length ? "#al-history" : "#al-title");
  } catch (err) {
    listError.value = errorMessage(err, "The activity wasn't deleted. Try again.");
  } finally {
    deleting.value = false;
  }
}

const describe = (a: ManualActivity) => {
  const when = toDate(a.startDate)?.toLocaleDateString() ?? "";
  return [a.sportType, when, describeBlocks(a.blocks)].filter(Boolean).join(" · ");
};

async function addBlock(): Promise<void> {
  const b = blankBlock();
  form.blocks.push(b);
  await nextTick();
  document.getElementById(`al-b${b.key}-ex-${b.stations[0].key}`)?.focus();
}
function removeBlock(i: number): void {
  form.blocks.splice(i, 1);
  focusNthOr("[data-remove-block]", i, "#al-add-block");
}
</script>

<template>
  <main class="al-page">
    <div class="al">
      <h1 id="al-title" tabindex="-1">Log activity</h1>
      <p class="meta">For things Strava doesn't see, like a gym session. Counts toward your goals and campaigns.</p>

      <form class="form" novalidate @submit.prevent="save">
        <div class="rb-field">
          <label for="al-sport">Sport</label>
          <select id="al-sport" v-model="form.sport" class="input" :aria-invalid="showErrors && !!errors.sport" aria-describedby="al-sport-error">
            <option value="" disabled>Choose…</option>
            <optgroup v-for="g in catalog.sportGroups" :key="g.group" :label="g.label">
              <option v-for="t in g.sportTypes" :key="t" :value="t">{{ t }}</option>
            </optgroup>
          </select>
          <span id="al-sport-error" class="field-error">{{ showErrors ? errors.sport : "" }}</span>
        </div>

        <div class="rb-field">
          <label for="al-name">Name <small>optional</small></label>
          <input id="al-name" v-model="form.name" class="input" maxlength="80" autocomplete="off" placeholder="Push day" />
        </div>

        <div class="rb-field">
          <label for="al-when">When</label>
          <input id="al-when" v-model="form.when" class="input" type="datetime-local" :aria-invalid="showErrors && !!errors.when" aria-describedby="al-when-error" />
          <span id="al-when-error" class="field-error">{{ showErrors ? errors.when : "" }}</span>
        </div>

        <div class="row">
          <div class="rb-field">
            <label for="al-duration">Time <small>optional</small></label>
            <input id="al-duration" class="input" :value="form.duration" inputmode="numeric" placeholder="45:00" @input="form.duration = maskDuration(($event.target as HTMLInputElement).value)" :aria-invalid="showErrors && !!errors.duration" aria-describedby="al-duration-error" />
            <span id="al-duration-error" class="field-error">{{ showErrors ? errors.duration : "" }}</span>
          </div>
          <div v-if="!hasBlocks" class="rb-field">
            <label for="al-distance">Distance (km) <small>optional</small></label>
            <input id="al-distance" v-model="form.distanceKm" class="input" inputmode="decimal" :aria-invalid="showErrors && !!errors.distance" aria-describedby="al-distance-error" />
            <span id="al-distance-error" class="field-error">{{ showErrors ? errors.distance : "" }}</span>
          </div>
        </div>

        <fieldset v-if="hasBlocks" class="rb-field blocks">
          <legend>Exercises</legend>
          <p class="hint">A block repeats its exercises for the rounds you set: straight sets are one exercise, a superset is two.</p>
          <BlockEditor
            v-for="(b, i) in form.blocks"
            :key="b.key"
            v-model="form.blocks[i]"
            :index="i"
            :id-prefix="`al-b${b.key}`"
            :errors="errors.blocks"
            :show-errors="showErrors"
            :can-remove="form.blocks.length > 1"
            @remove="removeBlock(i)"
          />
          <button id="al-add-block" type="button" class="pill" @click="addBlock">Add block</button>
        </fieldset>

        <p v-if="error" class="notice" role="alert">{{ error }}</p>
        <p v-if="saved" class="saved" role="status">{{ saved }}</p>
        <button type="submit" class="pill pill--primary" v-busy="saving">{{ saving ? "Saving…" : "Log activity" }}</button>
      </form>

      <section v-if="list.length || listError" class="history">
        <h2 id="al-history" tabindex="-1">Logged by you</h2>
        <p v-if="listError" class="field-error" role="alert">{{ listError }}</p>
        <ol>
          <li v-for="(a, i) in list" :key="String(a.id)">
            <span>
              <b>{{ a.name }}</b>
              <small>{{ describe(a) }}</small>
            </span>
            <span v-if="confirmDelete === a.id" class="confirm">
              <span>Delete this activity?</span>
              <span class="actions">
              <button v-focus="true" type="button" class="pill" @click="kept = a.id; confirmDelete = null">Keep</button>
              <button type="button" class="pill pill--primary" v-busy="deleting" @click="remove(a.id, i)">
                {{ deleting ? "Deleting…" : "Delete" }}<span class="visually-hidden"> {{ a.name }}</span>
              </button>
              </span>
            </span>
            <button v-else v-focus="kept === a.id" type="button" class="pill pill--quiet" data-delete @click="confirmDelete = a.id">Delete<span class="visually-hidden"> {{ a.name }}</span></button>
          </li>
        </ol>
      </section>
    </div>
  </main>
</template>

<style scoped>
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
.blocks {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-4);
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}
.blocks > section {
  align-self: stretch;
}
.hint {
  margin: 0;
  color: var(--ink-muted);
  font-size: 0.875rem;
}
.saved {
  margin: 0;
  font-weight: 700;
}
.notice {
  margin: 0;
  padding: var(--space-4) var(--space-5);
  border: 2px solid var(--line);
  border-radius: 10px;
  background: var(--cool-blue);
}
.history ol {
  list-style: none;
  margin: 0;
  padding: 0;
}
.history li {
  display: flex;
  flex-wrap: wrap;
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
.confirm {
  display: flex;
  flex: 1 1 16rem;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-2);
  font-weight: 700;
}
.confirm > span:first-child {
  margin-right: auto;
}
.actions {
  display: flex;
  gap: var(--space-2);
}
</style>
