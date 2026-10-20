<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { useCampaignsStore } from "../stores/campaigns";
import { useCampaignFeedStore } from "../stores/campaignFeed";
import { useGoalsStore } from "../stores/goals";
import { useCatalogStore } from "../stores/catalog";
import { toDate } from "../lib/timestamp";
import { dateRange, plural } from "../lib/schedule";
import { blankDraft, draftErrors, draftToInput, inputToDraft, unitDraft } from "../lib/goalInput";
import { clauseText } from "../lib/labels";
import { errorMessage } from "../lib/errors";
import CampaignBib from "../components/CampaignBib.vue";
import GoalBuilder from "../components/GoalBuilder.vue";

const props = defineProps<{ id: string }>();
const router = useRouter();
const auth = useAuthStore();
const store = useCampaignsStore();
const feedStore = useCampaignFeedStore();
const goals = useGoalsStore();
const catalog = useCatalogStore();

const loadError = ref("");
const saveError = ref("");
const saving = ref(false);
const showErrors = ref(false);
const goal = ref(blankDraft());
const copyFrom = ref("");

const campaign = computed(() => store.campaigns.find((c) => c.id === props.id));
const joined = computed(() => !!campaign.value?.joined);
const fmt = catalog.format;
const groupTarget = computed(() => {
  const c = campaign.value;
  return c?.metric !== undefined && c.targetValue !== undefined ? fmt(c.metric, c.targetValue) : "";
});

// Other members' goals, to start from (the leaderboard is members-only, so this is the add-a-goal path).
const friendGoals = computed(() =>
  feedStore
    .stateFor(props.id)
    .leaderboardGroups.flatMap((g) => g.entries.map((e) => ({ e, group: g.sportGroup })))
    .filter(({ e }) => e.userId !== auth.user?.id),
);

function copy(goalId: string): void {
  const found = friendGoals.value.find(({ e }) => e.goalId === goalId);
  if (!found) return;
  const { e, group } = found;
  goal.value = inputToDraft(
    { name: e.goalName, sport: group ? { case: "group", value: group } : { case: undefined }, clauses: e.clauseDefs },
    catalog.unitOf,
  );
}

const goalLine = computed(() => {
  const c = draftToInput(goal.value, catalog.unitOf).clauses.find((c) => c.value > 0);
  return c ? clauseText(c, catalog.metricInfo(c.metric), fmt) : "Set what you're chasing";
});

async function load(): Promise<void> {
  loadError.value = "";
  try {
    if (!auth.user) await auth.fetchCurrentUser();
    const [c] = await Promise.all([store.getCampaign(props.id), catalog.ensureLoaded()]);
    if (c.joined) await feedStore.fetchLeaderboard(c.id);
  } catch (err) {
    loadError.value = errorMessage(err, "We couldn't load this campaign. Try again.");
  }
}

onMounted(load);

async function save(withGoal: boolean): Promise<void> {
  showErrors.value = withGoal;
  if (withGoal && draftErrors(goal.value, catalog.unitOf).any) {
    await nextTick();
    document.querySelector<HTMLElement>('.cj-body [aria-invalid="true"]')?.focus();
    return;
  }
  const input = draftToInput(goal.value, catalog.unitOf);
  saving.value = true;
  saveError.value = "";
  try {
    if (joined.value) await goals.createGoal(input, props.id);
    else await store.joinCampaign(props.id, withGoal ? [input] : []);
    router.push(`/campaigns/${props.id}`);
  } catch (err) {
    saveError.value = errorMessage(err, joined.value ? "The goal wasn't saved. Try again." : "Joining didn't work. Try again.");
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <main class="cj-page">
    <div class="cj">
      <router-link :to="campaign ? `/campaigns/${campaign.id}` : '/'" class="back">
        <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M16 10H5M9 5l-5 5 5 5" /></svg>{{ campaign ? "Campaign" : "Campaigns" }}
      </router-link>

      <div v-if="loadError" class="notice" role="alert">
        <p>{{ loadError }}</p>
        <button type="button" class="pill" @click="load">Try again</button>
      </div>
      <p v-else-if="!campaign" class="quiet" aria-live="polite">Loading campaign…</p>

      <template v-else>
        <h1>{{ joined ? "Add a goal" : "Get your bib" }}</h1>
        <p class="meta">
          {{ campaign.name }} · {{ dateRange(toDate(campaign.startDate), toDate(campaign.endDate)) }} ·
          {{ plural(campaign.memberCount, "member") }}
        </p>

        <form class="cj-body" novalidate @submit.prevent="save(true)">
          <div class="preview" aria-hidden="true">
            <CampaignBib :band="[campaign.name, 'You']" :tab="groupTarget ? `Group target: ${groupTarget} together` : 'Ranked by how far along you are'">
              <span class="claim"><b>No. —</b><small>{{ joined ? "Ranked once it's saved" : "Assigned when you join" }}</small></span>
              <span class="claim-value">
                <b>{{ unitDraft(goal) ? fmt(unitDraft(goal)!.metric, 0) : "0%" }}</b>
                <small>{{ goalLine }}</small>
              </span>
            </CampaignBib>
          </div>

          <div>
            <p class="lede">
              {{ joined ? "Each goal gets its own place in the standings." : "Your Strava activities count from the moment you join." }}
              <template v-if="groupTarget"> They also add to the group's {{ groupTarget }}.</template>
            </p>

            <div v-if="joined && friendGoals.length" class="rb-field">
              <label for="cj-copy">Start from a friend's goal <small>optional</small></label>
              <select id="cj-copy" v-model="copyFrom" class="input" @change="copy(copyFrom)">
                <option value="">Start from scratch</option>
                <option v-for="{ e } in friendGoals" :key="e.goalId" :value="e.goalId">{{ e.userName }}: {{ e.goalName }}</option>
              </select>
            </div>

            <GoalBuilder v-model="goal" id-prefix="cj-goal" :show-errors="showErrors" />

            <p v-if="saveError" class="notice" role="alert">{{ saveError }}</p>
            <div class="actions">
              <button type="submit" class="pill pill--primary" :disabled="saving">
                {{ saving ? "Saving…" : joined ? "Add goal" : "Join campaign" }}
              </button>
              <button v-if="!joined" type="button" class="pill pill--quiet" :disabled="saving" @click="save(false)">Join without a goal</button>
            </div>
          </div>
        </form>
      </template>
    </div>
  </main>
</template>

<style scoped>
.cj-page {
  background: var(--paper);
  min-height: calc(100vh - 58px);
}
.cj {
  max-width: 1120px;
  margin: 0 auto;
  padding: 1rem 1rem 2rem;
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
  margin: 0.75rem 0 0;
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}
.meta {
  margin: 0.4rem 0 1.5rem;
  color: var(--ink-muted);
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
.preview {
  margin-bottom: 1.5rem;
}
/* No rank numeral here, so the bib body sizes to its content. */
.preview :deep(.main) {
  min-height: 0;
  align-items: center;
  padding-block: 0.9rem;
}
.claim {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  max-width: 11rem;
}
.claim b {
  font: 400 1.625rem/1.1 var(--font-display);
}
.claim small,
.claim-value small {
  color: var(--ink-muted);
  font-size: 0.8125rem;
}
.claim-value {
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  text-align: right;
}
.claim-value b {
  font: 400 1.625rem/1.1 var(--font-display);
  font-variant-numeric: tabular-nums;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
}
.lede {
  margin: 0 0 1.25rem;
  max-width: 50ch;
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
.quiet {
  color: var(--ink-muted);
  padding: 1rem 0;
}
svg {
  width: 1rem;
  height: 1rem;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

@media (min-width: 900px) {
  .cj {
    padding: 1.5rem 1.5rem 3rem;
  }
  .cj-body {
    display: grid;
    grid-template-columns: minmax(340px, 400px) minmax(0, 480px);
    column-gap: 3rem;
    align-items: start;
  }
  .preview {
    margin: 0;
  }
}
</style>
