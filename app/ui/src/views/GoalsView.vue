<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from "vue";
import { useGoalsStore } from "../stores/goals";
import { useCatalogStore } from "../stores/catalog";
import { ClauseScope, Comparator } from "../gen/fitter/v1/common_pb";
import type { Goal } from "../gen/fitter/v1/goal_pb";
import { blankDraft, draftErrors, draftToInput, inputToDraft } from "../lib/goalInput";
import { clauseText, sportSelectorLabel } from "../lib/labels";
import { DAY, dateRange, isoDay, localDay } from "../lib/schedule";
import { toDate } from "../lib/timestamp";
import { errorMessage } from "../lib/errors";
import CampaignBib from "../components/CampaignBib.vue";
import GoalBuilder from "../components/GoalBuilder.vue";

const store = useGoalsStore();
const catalog = useCatalogStore();
const fmt = catalog.format;

const listError = ref("");
// editing is "" when closed, "new" for a new goal, else the goal's id.
const editing = ref("");
const draft = ref(blankDraft());
const dates = reactive({ start: "", end: "", error: "" });
const showErrors = ref(false);
const saving = ref(false);
const saveError = ref("");
const confirmDelete = ref("");
const deleting = ref(false);
const deleteError = ref("");

async function load(): Promise<void> {
  listError.value = "";
  try {
    await Promise.all([store.fetchGoals(), catalog.ensureLoaded()]);
  } catch (err) {
    listError.value = errorMessage(err, "Goals didn't load. Try again.");
  }
}

onMounted(load);

async function open(g?: Goal): Promise<void> {
  showErrors.value = false;
  saveError.value = "";
  dates.error = "";
  editing.value = g?.id ?? "new";
  draft.value = g
    ? inputToDraft({ name: g.name, sport: g.sport?.selector ?? { case: undefined }, clauses: g.clauses }, catalog.unitOf)
    : blankDraft();
  const start = (g && toDate(g.periodStart)) || new Date();
  const end = (g && toDate(g.periodEnd)) || new Date(Date.now() + 30 * DAY);
  dates.start = isoDay(start);
  dates.end = isoDay(end);
  await nextTick();
  // The editor sits inside the list's v-for, so it's found by id rather than a template ref.
  document.getElementById("gl-heading")?.focus();
}

async function save(): Promise<void> {
  showErrors.value = true;
  dates.error = dates.end < dates.start ? "The end date can't be before the start date." : "";
  if (draftErrors(draft.value, catalog.unitOf).any || dates.error) {
    await nextTick();
    document.querySelector<HTMLElement>('.editor [aria-invalid="true"]')?.focus();
    return;
  }
  const input = {
    ...draftToInput(draft.value, catalog.unitOf),
    periodStart: localDay(dates.start),
    // The end day counts in full.
    periodEnd: new Date(localDay(dates.end).getTime() + DAY - 1),
  };
  saving.value = true;
  saveError.value = "";
  try {
    if (editing.value === "new") await store.createGoal(input);
    else await store.updateGoal(editing.value, input);
    editing.value = "";
  } catch (err) {
    saveError.value = errorMessage(err, "The goal wasn't saved. Try again.");
  } finally {
    saving.value = false;
  }
}

async function remove(id: string): Promise<void> {
  deleting.value = true;
  deleteError.value = "";
  try {
    await store.deleteGoal(id);
    confirmDelete.value = "";
  } catch (err) {
    deleteError.value = errorMessage(err, "The goal wasn't deleted. Try again.");
  } finally {
    deleting.value = false;
  }
}

const pct = (g: Goal) => Math.floor((g.progress?.progressRatio ?? 0) * 100 + 1e-9);

// A single "total at least" goal reads in units; anything else reads as the share done.
function headline(g: Goal): string {
  const [only, ...rest] = g.clauses;
  const p = g.progress?.clauses[0];
  return only && p && !rest.length && only.scope === ClauseScope.AGGREGATE && only.comparator === Comparator.GTE
    ? fmt(only.metric, p.currentValue)
    : "";
}

function subs(g: Goal): string[] {
  return g.clauses.map((c) => clauseText(c, catalog.metricInfo(c.metric), fmt));
}

// While a new goal is open, its editor takes the first slot of the list.
const items = computed(() => (editing.value === "new" ? [{ id: "new" } as Goal, ...sorted.value] : sorted.value));

const sorted = computed(() =>
  // Soonest to end first.
  [...store.goals].sort((a, b) => (toDate(a.periodEnd)?.getTime() ?? 0) - (toDate(b.periodEnd)?.getTime() ?? 0)),
);
</script>

<template>
  <main class="gl-page">
    <div class="gl">
      <header class="gl-head">
        <div>
          <h1>Goals</h1>
          <p class="meta">Private. Only you see these.</p>
        </div>
        <button v-if="!editing && store.goals.length" type="button" class="pill pill--primary" @click="open()">New goal</button>
      </header>

      <div v-if="listError" class="notice" role="alert">
        <p>{{ listError }}</p>
        <button type="button" class="pill" @click="load">Try again</button>
      </div>
      <p v-else-if="store.loading && !store.goals.length" class="quiet" aria-live="polite">Loading goals…</p>

      <CampaignBib v-else-if="!store.goals.length && !editing" :band="['No. —', 'Private']">
        <div class="unclaimed">
          <b>No goals yet</b>
          <small>Set a private target, like 200 km of riding this month. Your Strava activities count toward it.</small>
          <button type="button" class="pill pill--primary" @click="open()">New goal</button>
        </div>
      </CampaignBib>

      <ol v-else class="goals">
        <li v-for="g in items" :key="g.id" :class="{ 'is-editing': editing === g.id }">
          <form v-if="editing === g.id" class="editor" novalidate @submit.prevent="save">
            <h2 id="gl-heading" tabindex="-1">{{ editing === "new" ? "New goal" : "Edit goal" }}</h2>
            <GoalBuilder v-model="draft" id-prefix="gl-goal" :show-errors="showErrors" />
            <div class="dates">
              <div class="rb-field">
                <label for="gl-start">Starts</label>
                <input id="gl-start" v-model="dates.start" class="input" type="date" required />
              </div>
              <div class="rb-field">
                <label for="gl-end">Ends</label>
                <input
                  id="gl-end"
                  v-model="dates.end"
                  class="input"
                  type="date"
                  :min="dates.start"
                  required
                  :aria-invalid="!!dates.error"
                  aria-describedby="gl-dates-error"
                />
              </div>
            </div>
            <p id="gl-dates-error" class="field-error" role="alert">{{ dates.error }}</p>
            <p v-if="saveError" class="notice" role="alert">{{ saveError }}</p>
            <div class="actions">
              <button type="button" class="pill pill--quiet" @click="editing = ''">Cancel</button>
              <button type="submit" class="pill pill--primary" :disabled="saving">{{ saving ? "Saving…" : "Save goal" }}</button>
            </div>
          </form>
          <CampaignBib
            v-else
            :band="[g.name, g.progress?.satisfied ? 'Finisher' : sportSelectorLabel(g.sport, catalog.sportGroups)]"
            :numeral="`${pct(g)}%`"
            :numeral-label="`${pct(g)}% done`"
            :value="headline(g)"
            :subs="subs(g)"
            :finisher="g.progress?.satisfied"
          >
            <template #tab>
              <template v-if="confirmDelete === g.id">
                <span>Delete this goal?</span>
                <span class="tab-actions">
                  <button type="button" class="pill" @click="confirmDelete = ''">Keep</button>
                  <button type="button" class="pill pill--primary" :disabled="deleting" @click="remove(g.id)">
                    {{ deleting ? "Deleting…" : "Delete" }}
                  </button>
                </span>
              </template>
              <template v-else>
                <span>{{ dateRange(toDate(g.periodStart), toDate(g.periodEnd)) }}</span>
                <span class="tab-actions">
                  <button type="button" class="pill" @click="open(g)">Edit<span class="visually-hidden"> {{ g.name }}</span></button>
                  <button type="button" class="pill pill--quiet" @click="confirmDelete = g.id">
                    Delete<span class="visually-hidden"> {{ g.name }}</span>
                  </button>
                </span>
              </template>
            </template>
          </CampaignBib>
          <p v-if="deleteError && confirmDelete === g.id && editing !== g.id" class="field-error" role="alert">{{ deleteError }}</p>
        </li>
      </ol>
    </div>
  </main>
</template>

<style scoped>
.gl-page {
  background: var(--paper);
  min-height: calc(100vh - 58px);
}
.gl {
  max-width: 1120px;
  margin: 0 auto;
  padding: 1rem 1rem 2rem;
  font-family: var(--font-body);
  color: var(--ink);
}
.gl-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin: 0.5rem 0 1.25rem;
}
h1 {
  margin: 0;
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}
.meta {
  margin: 0.4rem 0 0;
  color: var(--ink-muted);
  font-size: 0.875rem;
}
h2 {
  margin: 0 0 1.25rem;
  font: 400 1.5rem/1.1 var(--font-display);
  text-transform: uppercase;
}
h2:focus {
  outline: none;
}

.is-editing {
  grid-column: 1 / -1;
}
.editor {
  max-width: 560px;
  padding-bottom: 1.5rem;
  border-bottom: 2px solid var(--ink);
}
.dates {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  column-gap: 1rem;
}
.dates .rb-field {
  margin-bottom: 0.5rem;
}
.field-error {
  margin: 0 0 1rem;
  font-weight: 700;
  font-size: 0.875rem;
}
.actions {
  display: flex;
  gap: 0.6rem;
}

.goals {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 340px), 1fr));
  gap: 1.25rem;
}
.tab-actions {
  display: flex;
  gap: 0.4rem;
}
.tab-actions .pill {
  min-height: 2.25rem;
  padding-block: 0.2rem;
}

.unclaimed {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.4rem;
}
.unclaimed b {
  font: 400 1.5rem/1.1 var(--font-display);
}
.unclaimed small {
  color: var(--ink-muted);
  font-size: 0.875rem;
  max-width: 46ch;
}
.unclaimed .pill {
  margin-top: 0.4rem;
}

.quiet {
  color: var(--ink-muted);
  padding: 1rem 0;
}
.notice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin: 0 0 1rem;
  padding: 0.75rem 1rem;
  border: 2px solid var(--ink);
  border-radius: 10px;
  background: var(--cool-blue);
  font-weight: 700;
}
.notice p {
  margin: 0;
}

@media (min-width: 900px) {
  .gl {
    padding: 1.5rem 1.5rem 3rem;
  }
}
</style>
