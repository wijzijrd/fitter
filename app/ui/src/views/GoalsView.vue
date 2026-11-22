<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useGoalsStore } from "../stores/goals";
import { useSyncStore } from "../stores/sync";
import { useCampaignsStore } from "../stores/campaigns";
import { useCatalogStore } from "../stores/catalog";
import { ClauseScope, Comparator, Recurrence } from "../gen/fitter/v1/common_pb";
import type { Goal } from "../gen/fitter/v1/goal_pb";
import { blankDraft, draftErrors, draftToInput, habitPreset, inputToDraft, recurrenceUnit, stretchPreset } from "../lib/goalInput";
import { clauseText, sportSelectorLabel } from "../lib/labels";
import { DAY, dateRange, isoDay, localDay } from "../lib/schedule";
import { toDate } from "../lib/timestamp";
import { errorMessage } from "../lib/errors";
import CampaignBib from "../components/CampaignBib.vue";
import GoalBuilder from "../components/GoalBuilder.vue";
import { focusNthOr } from "../lib/focus";

const store = useGoalsStore();
const campaigns = useCampaignsStore();
const route = useRoute();
const router = useRouter();
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
const kept = ref("");
const deleteError = ref("");
// "" = personal, else the campaign the goal is assigned to.
const assignTo = ref("");
const joinedCampaigns = computed(() => campaigns.campaigns.filter((c) => c.joined));
const assigned = computed(() => joinedCampaigns.value.find((c) => c.id === assignTo.value));
const campaignName = (id: string) => campaigns.campaigns.find((c) => c.id === id)?.name ?? "Campaign";

async function load(): Promise<void> {
  listError.value = "";
  try {
    await Promise.all([store.fetchAllGoals(), campaigns.fetchCampaigns(), catalog.ensureLoaded()]);
    // /goals/new?campaign=<id> opens the editor preselected on that campaign.
    if (route.name === "goal-new") await open(undefined, String(route.query.campaign ?? ""));
  } catch (err) {
    listError.value = errorMessage(err, "Goals didn't load. Try again.");
  }
}

onMounted(load);
// New Strava activities arrived: show them.
watch(() => useSyncStore().imported, load);

async function open(g?: Goal, campaignId = ""): Promise<void> {
  showErrors.value = false;
  saveError.value = "";
  dates.error = "";
  editing.value = g?.id ?? "new";
  assignTo.value = g ? (g.campaignId ?? "") : joinedCampaigns.value.some((c) => c.id === campaignId) ? campaignId : "";
  draft.value = g
    ? inputToDraft({ name: g.name, sport: g.sport?.selector ?? { case: undefined }, clauses: g.clauses, recurrence: g.recurrence }, catalog.unitOf)
    : blankDraft();
  const start = (g && toDate(g.periodStart)) || new Date();
  const end = (g && toDate(g.periodEnd)) || new Date(Date.now() + 30 * DAY);
  dates.start = isoDay(start);
  dates.end = isoDay(end);
  syncDates();
  await nextTick();
  // The editor sits inside the list's v-for, so it's found by id rather than a template ref.
  document.getElementById("gl-heading")?.focus();
}

// A campaign goal runs for the campaign's window, so the dates follow it.
function syncDates(): void {
  const c = assigned.value;
  if (!c) return;
  dates.start = isoDay(toDate(c.startDate) ?? new Date());
  dates.end = isoDay(toDate(c.endDate) ?? new Date());
}
watch(assignTo, syncDates);

function close(): void {
  editing.value = "";
  if (route.name === "goal-new") router.replace("/goals");
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
    if (editing.value === "new") await store.createGoal(input, assignTo.value || undefined);
    else await store.updateGoal(editing.value, input, assignTo.value);
    close();
  } catch (err) {
    saveError.value = errorMessage(err, "The goal wasn't saved. Try again.");
  } finally {
    saving.value = false;
  }
}

async function remove(id: string, i: number): Promise<void> {
  deleting.value = true;
  deleteError.value = "";
  try {
    await store.deleteGoal(id);
    confirmDelete.value = "";
    focusNthOr("[data-delete]", i, "#gl-title");
  } catch (err) {
    deleteError.value = errorMessage(err, "The goal wasn't deleted. Try again.");
  } finally {
    deleting.value = false;
  }
}

const pct = (g: Goal) => Math.floor((g.progress?.progressRatio ?? 0) * 100 + 1e-9);

function applyPreset(p: { draft: typeof draft.value; days: number }): void {
  draft.value = p.draft;
  if (assigned.value) return; // a campaign goal keeps the campaign's dates
  dates.start = isoDay(new Date());
  dates.end = isoDay(new Date(Date.now() + p.days * DAY));
}

// A single "total at least" goal reads in units; anything else reads as the share done.
function headline(g: Goal): string {
  const p0 = g.progress;
  if (g.recurrence > Recurrence.NONE && p0) return `${p0.periodsHit} of ${p0.periodsElapsed} ${recurrenceUnit[g.recurrence]}`;
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

// Personal goals first, then one run per campaign; soonest to end first within each.
const heading = (g: Goal, i: number): string => {
  if (g.id === "new") return "";
  const prev = items.value[i - 1];
  if (prev && prev.id !== "new" && (prev.campaignId ?? "") === (g.campaignId ?? "")) return "";
  return g.campaignId ? campaignName(g.campaignId) : "Personal";
};

const sorted = computed(() =>
  [...store.allGoals].sort(
    (a, b) =>
      (a.campaignId ? 1 : 0) - (b.campaignId ? 1 : 0) ||
      (a.campaignId ?? "").localeCompare(b.campaignId ?? "") ||
      (toDate(a.periodEnd)?.getTime() ?? 0) - (toDate(b.periodEnd)?.getTime() ?? 0),
  ),
);
</script>

<template>
  <main class="gl-page">
    <div class="gl">
      <header class="gl-head">
        <div>
          <h1 id="gl-title" tabindex="-1">Goals</h1>
          <p class="meta">Personal goals are private. Campaign goals show in that campaign's standings.</p>
        </div>
        <button v-if="!editing && store.allGoals.length" type="button" class="pill pill--primary" @click="open()">New goal</button>
      </header>

      <div v-if="listError" class="notice" role="alert">
        <p>{{ listError }}</p>
        <button type="button" class="pill" @click="load">Try again</button>
      </div>
      <p v-else-if="store.loading && !store.allGoals.length" class="quiet" aria-live="polite">Loading goals…</p>

      <CampaignBib v-else-if="!store.allGoals.length && !editing" class="empty" :band="['No. —', 'Private']">
        <div class="unclaimed">
          <b>No goals yet</b>
          <small>Set a private target, like 200 km of riding this month. Your Strava activities count toward it.</small>
          <button type="button" class="pill pill--primary" @click="open()">New goal</button>
        </div>
      </CampaignBib>

      <ol v-else class="goals">
        <li v-for="(g, i) in items" :key="g.id" :class="{ 'is-editing': editing === g.id, 'has-heading': !!heading(g, i) }">
          <h2 v-if="heading(g, i)" class="group-head">{{ heading(g, i) }}</h2>
          <form v-if="editing === g.id" class="editor" novalidate @submit.prevent="save">
            <h2 id="gl-heading" tabindex="-1">{{ editing === "new" ? "New goal" : "Edit goal" }}</h2>
            <div v-if="editing === 'new'" class="presets" role="group" aria-label="Start from a preset">
              <button type="button" class="pill" @click="applyPreset(habitPreset())">Habit: 3 days a week</button>
              <button type="button" class="pill" @click="applyPreset(stretchPreset())">Stretch: run a 5K</button>
            </div>
            <GoalBuilder v-model="draft" id-prefix="gl-goal" :show-errors="showErrors" />
            <div class="rb-field">
              <label for="gl-assign">Assign to</label>
              <select id="gl-assign" v-model="assignTo" class="input">
                <option value="">Personal (private)</option>
                <option v-for="c in joinedCampaigns" :key="c.id" :value="c.id">{{ c.name }}</option>
              </select>
              <small v-if="assigned">Runs for the campaign's dates. Others in the campaign see it.</small>
            </div>
            <div class="dates">
              <div class="rb-field">
                <label for="gl-start">Starts</label>
                <input id="gl-start" v-model="dates.start" class="input" type="date" :disabled="!!assigned" required />
              </div>
              <div class="rb-field">
                <label for="gl-end">Ends</label>
                <input
                  id="gl-end"
                  v-model="dates.end"
                  class="input"
                  type="date"
                  :min="dates.start"
                  :disabled="!!assigned"
                  required
                  :aria-invalid="!!dates.error"
                  aria-describedby="gl-dates-error"
                />
              </div>
            </div>
            <p id="gl-dates-error" class="field-error" role="alert">{{ dates.error }}</p>
            <p v-if="saveError" class="notice" role="alert">{{ saveError }}</p>
            <div class="actions">
              <button type="button" class="pill pill--quiet" @click="close">Cancel</button>
              <button type="submit" class="pill pill--primary" v-busy="saving">{{ saving ? "Saving…" : "Save goal" }}</button>
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
                  <button v-focus="true" type="button" class="pill" @click="kept = g.id; confirmDelete = ''">Keep</button>
                  <button type="button" class="pill pill--primary" v-busy="deleting" @click="remove(g.id, i)">
                    {{ deleting ? "Deleting…" : "Delete" }}
                  </button>
                </span>
              </template>
              <template v-else>
                <span>{{ dateRange(toDate(g.periodStart), toDate(g.periodEnd)) }}</span>
                <span class="tab-actions">
                  <button type="button" class="pill" @click="open(g)">Edit<span class="visually-hidden"> {{ g.name }}</span></button>
                  <button v-focus="kept === g.id" type="button" class="pill pill--quiet" data-delete @click="confirmDelete = g.id">
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
.gl {
  max-width: 1120px;
  margin: 0 auto;
  padding: var(--space-5) var(--gutter) var(--space-8);
  font-family: var(--font-body);
  color: var(--ink);
}
.gl-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-5);
  margin: var(--space-3) 0 var(--space-6);
}
h1 {
  margin: 0;
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}
.meta {
  margin: var(--space-2) 0 0;
  color: var(--ink-muted);
  font-size: 0.875rem;
}
h2 {
  margin: 0 0 var(--space-6);
  font: 400 1.5rem/1.1 var(--font-display);
  text-transform: uppercase;
}
.group-head {
  margin: var(--space-5) 0 var(--space-4);
  font-size: 1.125rem;
}

.is-editing {
  grid-column: 1 / -1;
}
.editor {
  max-width: 560px;
  padding-bottom: var(--space-7);
  border-bottom: 2px solid var(--line);
}
.presets {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-bottom: var(--space-6);
}
/* A bib, not a banner: the empty state keeps a goal bib's proportions on wide screens. */
.empty {
  max-width: 560px;
}
.dates {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
  column-gap: var(--space-5);
}
.dates .rb-field {
  margin-bottom: var(--space-3);
}
.field-error {
  margin: 0 0 var(--space-5);
  font-weight: 700;
  font-size: 0.875rem;
}
.actions {
  display: flex;
  gap: var(--space-3);
}

.goals {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 340px), 1fr));
  gap: var(--space-6);
}
.tab-actions {
  display: flex;
  gap: var(--space-2);
}
.tab-actions .pill {
  min-height: 2.25rem;
  padding-block: 0.2rem;
}
@media (pointer: coarse) {
  .tab-actions .pill {
    min-height: 2.75rem;
  }
}

.unclaimed {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-2);
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
  margin-top: var(--space-2);
}

.quiet {
  color: var(--ink-muted);
  padding: var(--space-5) 0;
}
.notice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  margin: 0 0 var(--space-5);
  padding: var(--space-4) var(--space-5);
  border: 2px solid var(--line);
  border-radius: 10px;
  background: var(--cool-blue);
  font-weight: 700;
}
.notice p {
  margin: 0;
}

@media (min-width: 900px) {
  .gl {
    padding: var(--space-7) var(--gutter) var(--space-9);
  }
}
</style>
