<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useCampaignsStore } from "../stores/campaigns";
import { ActivityMetric, CampaignMode } from "../gen/fitter/v1/common_pb";
import { metricLabels, targetInputUnits, formatMetricValue } from "../lib/labels";
import { DAY, plural, dateRange } from "../lib/schedule";
import { errorMessage } from "../lib/errors";
import CampaignBib from "../components/CampaignBib.vue";

const router = useRouter();
const store = useCampaignsStore();

const steps = ["Name it", "Set the window", "Set the rules"] as const;
const step = ref(0);
const heading = ref<HTMLElement | null>(null);
const creating = ref(false);
const createError = ref("");
const errors = reactive({ name: "", dates: "", target: "" });

// Dates are typed as local calendar days; parsing "YYYY-MM-DD" as UTC would shift them a day west of Greenwich.
const isoDay = (d: Date) => new Date(d.getTime() - d.getTimezoneOffset() * 60_000).toISOString().slice(0, 10);
const localDay = (s: string) => new Date(`${s}T00:00:00`);

const form = reactive({
  name: "",
  description: "",
  mode: CampaignMode.PER_MEMBER,
  metric: ActivityMetric.DISTANCE,
  target: 100,
  start: isoDay(new Date()),
  end: isoDay(new Date(Date.now() + 30 * DAY)),
});

const metrics = [ActivityMetric.DISTANCE, ActivityMetric.ELEVATION, ActivityMetric.TIME, ActivityMetric.COUNT];
const unit = computed(() => targetInputUnits[form.metric]);
const rawTarget = computed(() => (Number(form.target) || 0) * unit.value.factor);
const fmt = (v: number) => formatMetricValue(form.metric, v);
// The preview holds the last valid target while the field is empty or invalid.
const lastTarget = ref(100 * targetInputUnits[ActivityMetric.DISTANCE].factor);
watch(rawTarget, (v) => v > 0 && (lastTarget.value = v));
const shownTarget = computed(() => (rawTarget.value > 0 ? rawTarget.value : lastTarget.value));
const pooled = computed(() => form.mode === CampaignMode.POOLED);
const days = computed(() => Math.max(0, Math.round((localDay(form.end).getTime() - localDay(form.start).getTime()) / DAY) + 1));
const range = computed(() => dateRange(localDay(form.start), localDay(form.end)));

const preview = computed(() => {
  const name = form.name.trim() || "Your campaign";
  const length = plural(days.value, "day");
  if (pooled.value)
    return {
      band: [name, "Group"] as [string, string],
      numeral: "0%",
      value: fmt(0),
      subs: [`of ${fmt(shownTarget.value)}`, length],
      tab: `${fmt(shownTarget.value)} together · ${range.value}`,
    };
  return {
    band: [name, "You"] as [string, string],
    numeral: "1",
    value: fmt(0),
    subs: [`${fmt(shownTarget.value)} to go`, length],
    tab: `${fmt(shownTarget.value)} each · ${range.value}`,
  };
});

const summary = computed(
  () =>
    `${form.name.trim()}: ${fmt(shownTarget.value)} of ${metricLabels[form.metric].toLowerCase()} ${pooled.value ? "together" : "each"}, ${range.value} (${plural(days.value, "day")}).`,
);

function validate(): boolean {
  errors.name = step.value === 0 && !form.name.trim() ? "Give the campaign a name." : "";
  errors.dates = step.value === 1 && form.end < form.start ? "The end date can't be before the start date." : "";
  errors.target = step.value === 2 && !(rawTarget.value > 0) ? "Set a target above zero." : "";
  return !errors.name && !errors.dates && !errors.target;
}

async function go(to: number): Promise<void> {
  step.value = to;
  await nextTick();
  heading.value?.focus();
}

async function next(): Promise<void> {
  if (!validate()) {
    await nextTick();
    const field = document.querySelector<HTMLElement>('.steps [aria-invalid="true"]');
    // Centered, so the field and its error clear the sticky action bar on phones.
    field?.scrollIntoView({ block: "center" });
    field?.focus({ preventScroll: true });
    return;
  }
  if (step.value < steps.length - 1) return go(step.value + 1);
  creating.value = true;
  createError.value = "";
  try {
    const c = await store.createCampaign({
      name: form.name.trim(),
      description: form.description.trim(),
      mode: form.mode,
      metric: form.metric,
      targetValue: rawTarget.value,
      startDate: localDay(form.start),
      // The end day counts in full: the server includes activities up to end_date.
      endDate: new Date(localDay(form.end).getTime() + DAY - 1),
    });
    router.push(`/campaigns/${c.id}`);
  } catch (err) {
    createError.value = errorMessage(err, "The campaign wasn't created. Try again.");
  } finally {
    creating.value = false;
  }
}
</script>

<template>
  <main class="cc-page">
    <div class="cc">
      <router-link to="/" class="back">
        <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M16 10H5M9 5l-5 5 5 5" /></svg>Campaigns
      </router-link>
      <h1>New campaign</h1>

      <div class="cc-body">
        <div class="preview" aria-hidden="true">
          <CampaignBib v-bind="preview" />
        </div>

        <form class="steps" novalidate @submit.prevent="next">
          <h2 ref="heading" tabindex="-1"><span class="visually-hidden">Step {{ step + 1 }} of {{ steps.length }}: </span>{{ steps[step] }}</h2>

          <template v-if="step === 0">
            <div class="rb-field">
              <label for="cc-name">Name</label>
              <input
                id="cc-name"
                v-model="form.name"
                class="input"
                maxlength="80"
                autocomplete="off"
                :aria-invalid="!!errors.name"
                aria-describedby="cc-name-error"
              />
              <span id="cc-name-error" class="field-error" role="alert">{{ errors.name }}</span>
            </div>
            <div class="rb-field">
              <label for="cc-desc">Description <small>optional</small></label>
              <textarea id="cc-desc" v-model="form.description" class="input" rows="3" maxlength="500" placeholder="What the crew is chasing, and any house rules." />
            </div>
          </template>

          <template v-else-if="step === 1">
            <div class="dates">
              <div class="rb-field">
                <label for="cc-start">Starts</label>
                <input id="cc-start" v-model="form.start" class="input" type="date" required />
              </div>
              <div class="rb-field">
                <label for="cc-end">Ends</label>
                <input
                  id="cc-end"
                  v-model="form.end"
                  class="input"
                  type="date"
                  :min="form.start"
                  required
                  :aria-invalid="!!errors.dates"
                  aria-describedby="cc-dates-note"
                />
              </div>
            </div>
            <p id="cc-dates-note" class="note" :class="{ 'field-error': errors.dates }" role="status">
              {{ errors.dates || `${plural(days, "day")} of racing, both dates included.` }}
            </p>
          </template>

          <template v-else>
            <fieldset class="rb-field">
              <legend>How it's scored</legend>
              <label class="choice">
                <input v-model.number="form.mode" type="radio" name="mode" :value="CampaignMode.PER_MEMBER" />
                <span><b>Each on their own</b><small>Everyone chases the target and is ranked by progress. Members can set their own target when they join.</small></span>
              </label>
              <label class="choice">
                <input v-model.number="form.mode" type="radio" name="mode" :value="CampaignMode.POOLED" />
                <span><b>Together</b><small>One shared target. Everyone's activities add up to it.</small></span>
              </label>
            </fieldset>

            <fieldset class="rb-field">
              <legend>What counts</legend>
              <div class="metrics">
                <label v-for="m in metrics" :key="m" class="choice choice--pill">
                  <input v-model.number="form.metric" type="radio" name="metric" :value="m" />
                  <span>{{ metricLabels[m] }}</span>
                </label>
              </div>
            </fieldset>

            <div class="rb-field">
              <label for="cc-target">{{ pooled ? "Group target" : "Target for each member" }} ({{ unit.unit }})</label>
              <input
                id="cc-target"
                v-model.number="form.target"
                class="input input--target"
                type="number"
                min="1"
                step="any"
                inputmode="decimal"
                :aria-invalid="!!errors.target"
                aria-describedby="cc-target-error"
              />
              <span id="cc-target-error" class="field-error" role="alert">{{ errors.target }}</span>
            </div>

            <p class="summary">{{ summary }}</p>
          </template>

          <p v-if="createError" class="notice" role="alert">{{ createError }}</p>

          <div class="bar">
            <button v-if="step > 0" type="button" class="pill" @click="go(step - 1)">
              <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M16 10H5M9 5l-5 5 5 5" /></svg>Back
            </button>
            <router-link v-else to="/" class="pill pill--quiet">Cancel</router-link>
            <span class="count" aria-hidden="true">{{ step + 1 }} / {{ steps.length }}</span>
            <button type="submit" class="pill pill--primary" :disabled="creating">
              <template v-if="step < steps.length - 1">
                Next<svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h11M11 5l5 5-5 5" /></svg>
              </template>
              <template v-else>{{ creating ? "Creating…" : "Create campaign" }}</template>
            </button>
          </div>
        </form>
      </div>
    </div>
  </main>
</template>

<style scoped>
.cc-page {
  background: var(--paper);
  min-height: calc(100vh - 58px);
}
.cc {
  max-width: 1120px;
  margin: 0 auto;
  padding: 1rem 1rem 0;
  font-family: var(--font-body);
  color: var(--ink);
}
.back {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  color: var(--ink-muted);
  font-size: 0.875rem;
  font-weight: 600;
}
.back:hover {
  color: var(--ink);
}
.back:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: 2px;
}
h1 {
  margin: 0.75rem 0 1.25rem;
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}

.preview {
  margin-bottom: 1.75rem;
}
.preview :deep(.main) {
  min-height: 5.5rem;
}

.count {
  align-self: center;
  color: var(--ink-muted);
  font: 400 0.875rem var(--font-display);
  font-variant-numeric: tabular-nums;
}
h2 {
  margin: 0 0 1.25rem;
  font: 400 1.5rem/1.1 var(--font-display);
  text-transform: uppercase;
}
h2:focus {
  outline: none;
}

fieldset.rb-field {
  border: 0;
  margin-inline: 0;
  padding: 0;
  gap: 0.5rem;
}
legend {
  margin-bottom: 0.5rem;
}
.dates {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  column-gap: 1rem;
}
.dates .rb-field {
  margin-bottom: 0.5rem;
}
.note {
  margin: 0 0 1.25rem;
  color: var(--ink-muted);
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}
.note.field-error {
  color: var(--ink);
  font-weight: 700;
}

.choice {
  display: flex;
  align-items: flex-start;
  gap: 0.65rem;
  border: 2px solid var(--ink);
  border-radius: 10px;
  padding: 0.7rem 0.9rem;
  cursor: pointer;
  transition: background-color 160ms var(--ease-out-expo);
}
@media (hover: hover) {
  .choice:hover {
    background: var(--cool-blue-soft);
  }
}
.choice:has(input:checked) {
  background: var(--cool-blue);
}
.choice:has(input:focus-visible) {
  outline: 3px solid var(--ink);
  outline-offset: 2px;
}
.choice input {
  margin: 0.2rem 0 0;
  accent-color: var(--ink);
  width: 1.05rem;
  height: 1.05rem;
  flex: none;
}
.choice span {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}
.choice b {
  font-size: 1rem;
}
.choice small {
  max-width: 50ch;
}
.metrics {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.choice--pill {
  align-items: center;
  border-radius: 999px;
  padding: 0.45rem 0.9rem 0.45rem 0.7rem;
  font-weight: 700;
  font-size: 0.9375rem;
}
.choice--pill input {
  margin: 0;
}
.input--target {
  max-width: 12rem;
  font: 400 1.625rem/1.1 var(--font-display);
}
.summary {
  margin: 0 0 1.25rem;
  max-width: 60ch;
  font-size: 0.9375rem;
  font-variant-numeric: tabular-nums;
}
.notice {
  margin: 0 0 1rem;
  padding: 0.75rem 1rem;
  border: 2px solid var(--ink);
  border-radius: 10px;
  background: var(--cool-blue);
  font-weight: 700;
}

.bar {
  position: sticky;
  bottom: 0;
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  margin: 0 -1rem;
  padding: 0.75rem 1rem calc(0.75rem + env(safe-area-inset-bottom));
  border-top: 2px solid var(--ink);
  background: var(--paper);
}
.bar .pill {
  text-decoration: none;
}
.bar svg {
  width: 1.1rem;
  height: 1.1rem;
}

svg {
  width: 1rem;
  height: 1rem;
  flex: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

@media (min-width: 900px) {
  .cc {
    padding: 1.5rem 1.5rem 3rem;
  }
  .cc-body {
    display: grid;
    grid-template-columns: minmax(340px, 400px) minmax(0, 560px);
    column-gap: 3rem;
    align-items: start;
  }
  .preview {
    position: sticky;
    top: 1.5rem;
    margin: 0;
  }
  .preview :deep(.main) {
    min-height: 7rem;
  }
  .bar {
    position: static;
    margin: 0.5rem 0 0;
    padding: 1.25rem 0 0;
  }
}
</style>
