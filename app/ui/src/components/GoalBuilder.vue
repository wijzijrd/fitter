<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useCatalogStore } from "../stores/catalog";
import { ClauseScope, Comparator, MetricUnit } from "../gen/fitter/v1/common_pb";
import type { ActivityMetric } from "../gen/fitter/v1/common_pb";
import type { ClauseDraft, GoalDraft } from "../lib/goalInput";
import { blankClause, draftErrors } from "../lib/goalInput";
import { comparatorLabels, fasterLabels, scopeLabels } from "../lib/labels";
import { editUnitHint, isDurationUnit } from "../lib/metricFormat";

const draft = defineModel<GoalDraft>({ required: true });
const props = defineProps<{
  // idPrefix keeps label/input ids unique when a page shows more than one builder.
  idPrefix: string;
  // Errors only show once the parent has tried to submit.
  showErrors: boolean;
}>();

const catalog = useCatalogStore();
onMounted(() => catalog.ensureLoaded().catch(() => {}));

const id = (s: string) => `${props.idPrefix}-${s}`;
const errors = computed(() => draftErrors(draft.value, catalog.unitOf));

const OTHER = "__other__";
const sportValue = computed(() => {
  const s = draft.value.sport;
  return s.case === "group" ? s.value : s.case === "sportType" ? OTHER : "";
});
function onSport(v: string): void {
  draft.value.sport = v === "" ? { case: undefined } : v === OTHER ? { case: "sportType", value: "" } : { case: "group", value: v };
}

// A metric can be curated to some sport groups; with no group chosen every metric applies.
const metricOptions = computed(() => {
  const s = draft.value.sport;
  return catalog.metrics.filter(
    (m) => s.case !== "group" || !m.applicableSportGroups.length || m.applicableSportGroups.includes(s.value),
  );
});
// The clause's current metric stays listed even if a later sport change rules it out.
const optionsFor = (c: ClauseDraft) =>
  metricOptions.value.some((m) => m.metric === c.metric)
    ? metricOptions.value
    : [...catalog.metrics.filter((m) => m.metric === c.metric), ...metricOptions.value];

function scopesFor(metric: ActivityMetric): ClauseScope[] {
  const info = catalog.metricInfo(metric);
  const out: ClauseScope[] = [];
  if (info?.supportsAggregateScope ?? true) out.push(ClauseScope.AGGREGATE);
  if (info?.supportsSingleScope) out.push(ClauseScope.SINGLE);
  return out;
}

function onMetric(c: ClauseDraft, metric: ActivityMetric): void {
  c.metric = metric;
  const scopes = scopesFor(metric);
  if (!scopes.includes(c.scope)) c.scope = scopes[0] ?? ClauseScope.AGGREGATE;
  c.comparator = defaultComparator(metric);
  c.valueText = "";
}

const lowerIsBetter = (metric: ActivityMetric) => !!catalog.metricInfo(metric)?.lowerIsBetter;
// Pace-like metrics are chased downward.
const defaultComparator = (metric: ActivityMetric) => (lowerIsBetter(metric) ? Comparator.LTE : Comparator.GTE);
const comparators = (metric: ActivityMetric) =>
  lowerIsBetter(metric) ? [Comparator.LTE, Comparator.GTE] : [Comparator.GTE, Comparator.LTE];
const comparatorLabel = (metric: ActivityMetric, cmp: Comparator) =>
  (lowerIsBetter(metric) ? fasterLabels : comparatorLabels)[cmp];

function add(): void {
  const c = blankClause(metricOptions.value[0]?.metric);
  c.comparator = defaultComparator(c.metric);
  draft.value.clauses.push(c);
}
function remove(key: number): void {
  draft.value.clauses = draft.value.clauses.filter((c) => c.key !== key);
}

const isDuration = (c: ClauseDraft) => isDurationUnit(catalog.unitOf(c.metric));
const isPace = (c: ClauseDraft) => catalog.unitOf(c.metric) === MetricUnit.SECONDS_PER_KM;
// Durations are typed as clock times, so the format goes in the placeholder and the unit stays beside the field.
const placeholder = (c: ClauseDraft) => (isPace(c) ? "m:ss" : isDuration(c) ? "h:mm:ss" : "");
const unitText = (c: ClauseDraft) => (isPace(c) ? "/km" : isDuration(c) ? "" : editUnitHint(catalog.unitOf(c.metric)));
</script>

<template>
  <div class="gb">
    <div class="rb-field">
      <label :for="id('name')">Goal name</label>
      <input
        :id="id('name')"
        v-model="draft.name"
        class="input"
        maxlength="80"
        autocomplete="off"
        placeholder="Spring base miles"
        :aria-invalid="showErrors && !!errors.name"
        :aria-describedby="id('name-error')"
      />
      <span :id="id('name-error')" class="field-error" role="alert">{{ showErrors ? errors.name : "" }}</span>
    </div>

    <div class="rb-field">
      <label :for="id('sport')">Sport</label>
      <select :id="id('sport')" class="input" :value="sportValue" @change="onSport(($event.target as HTMLSelectElement).value)">
        <option value="">All sports</option>
        <option v-for="g in catalog.sportGroups" :key="g.group" :value="g.group">{{ g.label }}</option>
        <option :value="OTHER">Other sport…</option>
      </select>
      <template v-if="draft.sport.case === 'sportType'">
        <label :for="id('sport-type')" class="visually-hidden">Strava sport type</label>
        <input
          :id="id('sport-type')"
          v-model.trim="draft.sport.value"
          class="input"
          autocomplete="off"
          placeholder="Strava sport type, e.g. TrailRun"
        />
      </template>
    </div>

    <fieldset class="rb-field clauses">
      <legend>What counts</legend>
      <p v-if="!catalog.loaded" class="quiet">Loading metrics…</p>
      <ol v-else class="rows">
        <li v-for="(c, i) in draft.clauses" :key="c.key" class="clause">
          <div class="clause-head">
            <label :for="id(`metric-${c.key}`)" class="visually-hidden">Condition {{ i + 1 }} metric</label>
            <select
              :id="id(`metric-${c.key}`)"
              class="input"
              :value="c.metric"
              @change="onMetric(c, Number(($event.target as HTMLSelectElement).value))"
            >
              <option v-for="m in optionsFor(c)" :key="m.metric" :value="m.metric">{{ m.label }}</option>
            </select>
            <button v-if="draft.clauses.length > 1" type="button" class="pill pill--quiet" @click="remove(c.key)">
              Remove<span class="visually-hidden"> condition {{ i + 1 }}</span>
            </button>
          </div>

          <div class="clause-rules">
            <div v-if="scopesFor(c.metric).length > 1" class="seg" role="radiogroup" :aria-label="`Condition ${i + 1} scope`">
              <label v-for="s in scopesFor(c.metric)" :key="s" class="choice choice--pill">
                <input v-model="c.scope" type="radio" :name="id(`scope-${c.key}`)" :value="s" />
                <span>{{ scopeLabels[s] }}</span>
              </label>
            </div>
            <span v-else class="fixed">{{ scopeLabels[c.scope] }}</span>

            <div class="seg" role="radiogroup" :aria-label="`Condition ${i + 1} comparison`">
              <label v-for="cmp in comparators(c.metric)" :key="cmp" class="choice choice--pill">
                <input v-model="c.comparator" type="radio" :name="id(`cmp-${c.key}`)" :value="cmp" />
                <span>{{ comparatorLabel(c.metric, cmp) }}</span>
              </label>
            </div>

            <span class="value">
              <label :for="id(`value-${c.key}`)" class="visually-hidden">Condition {{ i + 1 }} value</label>
              <input
                :id="id(`value-${c.key}`)"
                v-model="c.valueText"
                class="input input--value"
                autocomplete="off"
                :inputmode="isDuration(c) ? 'text' : 'decimal'"
                :placeholder="placeholder(c)"
                :aria-invalid="showErrors && !!errors.clauses[i]"
                :aria-describedby="id(`value-${c.key}-error`)"
              />
              <span v-if="unitText(c)" class="unit" aria-hidden="true">{{ unitText(c) }}</span>
            </span>
          </div>
          <span :id="id(`value-${c.key}-error`)" class="field-error" role="alert">{{ showErrors ? errors.clauses[i] : "" }}</span>
        </li>
      </ol>
      <button v-if="catalog.loaded" type="button" class="pill add" @click="add">
        <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M10 4v12M4 10h12" /></svg>Add condition
      </button>
      <small v-if="draft.clauses.length > 1">All conditions must be met.</small>
    </fieldset>
  </div>
</template>

<style scoped>
.clauses {
  border: 0;
  margin-inline: 0;
  padding: 0;
}
legend {
  margin-bottom: 0.25rem;
}
.rows {
  list-style: none;
  margin: 0 0 0.75rem;
  padding: 0;
}
.clause {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.8rem 0;
  border-bottom: 1px solid var(--hairline);
}
.clause:first-child {
  padding-top: 0.4rem;
}
.clause-head {
  display: flex;
  gap: 0.5rem;
}
.clause-head select {
  flex: 1;
  min-width: 0;
}
.clause-rules {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 0.75rem;
}
.seg {
  display: flex;
  gap: 0.35rem;
}
.fixed {
  font-weight: 700;
  font-size: 0.9375rem;
}
.value {
  flex-basis: 100%;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}
.input--value {
  width: 7.5rem;
  font: 400 1.25rem/1.1 var(--font-display);
}
.unit {
  font-weight: 700;
  font-size: 0.9375rem;
}
.add {
  align-self: flex-start;
}
.quiet {
  color: var(--ink-muted);
  margin: 0.25rem 0 0.75rem;
}
svg {
  width: 1rem;
  height: 1rem;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
}
</style>
