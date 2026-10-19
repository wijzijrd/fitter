<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { useCampaignsStore } from "../stores/campaigns";
import { useCampaignFeedStore } from "../stores/campaignFeed";
import { useGoalsStore } from "../stores/goals";
import { metricLabels, formatMetricValue, activityMetricValue } from "../lib/labels";
import { toDate, formatRelative } from "../lib/timestamp";
import TimelineChart from "../components/charts/TimelineChart.vue";
import BibStack from "../components/BibStack.vue";
import { CampaignMode } from "../gen/fitter/v1/common_pb";
import { errorMessage } from "../lib/errors";
import poweredByStrava from "../assets/strava/api_logo_pwrdBy_strava_horiz_black.svg";

const props = defineProps<{ id: string }>();
const router = useRouter();

const auth = useAuthStore();
const campaignsStore = useCampaignsStore();
const feedStore = useCampaignFeedStore();
const goalsStore = useGoalsStore();

type Tab = "standings" | "feed" | "timeline";
const tabs: { id: Tab; label: string }[] = [
  { id: "standings", label: "Standings" },
  { id: "feed", label: "Feed" },
  { id: "timeline", label: "Timeline" },
];
const tab = ref<Tab>("standings");
const loadError = ref("");
const joinError = ref("");
const joining = ref(false);
const leaving = ref(false);
const confirmLeave = ref(false);
const leaveError = ref("");
const stack = ref<InstanceType<typeof BibStack> | null>(null);

const campaign = computed(() => campaignsStore.campaigns.find((c) => c.id === props.id));
const feed = computed(() => feedStore.stateFor(props.id));
const meId = computed(() => auth.user?.id);
const pooled = computed(() => campaign.value?.mode === CampaignMode.POOLED);
const fmt = (v: number) => formatMetricValue(campaign.value!.metric, v);

const DAY = 86_400_000;
const schedule = computed(() => {
  const c = campaign.value;
  const start = toDate(c?.startDate);
  const end = toDate(c?.endDate);
  if (!start || !end) return { meta: "", note: "" };
  const now = Date.now();
  if (now < start.getTime()) {
    const days = Math.ceil((start.getTime() - now) / DAY);
    return { meta: `Starts in ${days} ${days === 1 ? "day" : "days"}`, note: "" };
  }
  if (now > end.getTime()) return { meta: "Final standings", note: "" };
  const total = Math.max(1, Math.round((end.getTime() - start.getTime()) / DAY));
  const day = Math.min(total, Math.floor((now - start.getTime()) / DAY) + 1);
  const left = Math.ceil((end.getTime() - now) / DAY);
  return { meta: `Day ${day} of ${total}`, note: `${left} ${left === 1 ? "day" : "days"} left` };
});

const lastActive = computed(() => {
  const m = new Map<string, Date>();
  for (const a of feed.value.activities) {
    const d = toDate(a.startDate);
    const seen = m.get(a.userId);
    if (d && (!seen || d > seen)) m.set(a.userId, d);
  }
  return m;
});

const privateGoal = computed(() => {
  const c = campaign.value;
  if (!c) return null;
  const now = new Date();
  const g = goalsStore.goals.find((g) => {
    const s = toDate(g.periodStart);
    const e = toDate(g.periodEnd);
    return g.metric === c.metric && s && e && s <= now && now <= e;
  });
  if (!g) return null;
  const pct = g.targetValue > 0 ? Math.round((g.progressValue / g.targetValue) * 100) : 0;
  const by = toDate(g.periodEnd)!.toLocaleDateString(undefined, { day: "numeric", month: "short" });
  return `Your goal: ${fmt(g.targetValue)} by ${by} · ${pct}%`;
});

async function load(): Promise<void> {
  loadError.value = "";
  try {
    if (!auth.user) await auth.fetchCurrentUser();
    const c = await campaignsStore.getCampaign(props.id);
    // Standings, feed and timeline are members-only on the server.
    if (c.joined) await feedStore.fetchAll(props.id);
  } catch (err) {
    loadError.value = errorMessage(err, "We couldn't load this campaign. Check your connection and try again.");
  }
}

async function join(): Promise<void> {
  // Per-member campaigns let you set your own target first.
  if (!pooled.value) return void router.push(`/campaigns/${props.id}/join`);
  joining.value = true;
  joinError.value = "";
  try {
    await campaignsStore.joinCampaign(props.id);
    await feedStore.fetchAll(props.id);
  } catch (err) {
    joinError.value = errorMessage(err, "Joining didn't work. Try again in a moment.");
  } finally {
    joining.value = false;
  }
}

async function leave(): Promise<void> {
  leaving.value = true;
  leaveError.value = "";
  try {
    await campaignsStore.leaveCampaign(props.id);
    router.push("/");
  } catch (err) {
    leaveError.value = errorMessage(err, "Leaving didn't work. Try again.");
    leaving.value = false;
  }
}

function selectTab(ev: KeyboardEvent): void {
  const i = tabs.findIndex((t) => t.id === tab.value);
  const next = ev.key === "ArrowRight" ? i + 1 : ev.key === "ArrowLeft" ? i - 1 : -2;
  if (next === -2) return;
  tab.value = tabs[(next + tabs.length) % tabs.length].id;
  (document.getElementById(`tab-${tab.value}`) as HTMLElement | null)?.focus();
}

function rowNote(userId: string, progress: number): string {
  const last = lastActive.value.get(userId);
  // The feed only carries recent activities, so progress without one in it is just older.
  if (last) return `Active ${formatRelative(last)}`;
  return progress > 0 ? "" : "No activity yet";
}

onMounted(() => {
  load();
  goalsStore.fetchGoals().catch(() => {});
});
</script>

<template>
  <main class="cd-page">
    <div class="cd">
      <router-link to="/" class="back">
        <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M16 10H5M9 5l-5 5 5 5" /></svg>Campaigns
      </router-link>

      <div v-if="loadError" class="notice" role="alert">
        <p>{{ loadError }}</p>
        <button type="button" class="pill" @click="load">Try again</button>
      </div>

      <p v-else-if="!campaign" class="quiet" aria-live="polite">Loading campaign…</p>

      <template v-else>
        <header class="cd-head">
          <h1>{{ campaign.name }}</h1>
          <p class="meta">
            {{ pooled ? "Pooled" : "Per member" }} · {{ metricLabels[campaign.metric] }}
            <template v-if="schedule.meta"> · {{ schedule.meta }}</template>
            · {{ campaign.memberCount }} {{ campaign.memberCount === 1 ? "member" : "members" }}
          </p>
          <p v-if="campaign.description" class="desc">{{ campaign.description }}</p>
        </header>

        <div class="cd-body">
          <div v-if="campaign.joined" class="cd-tabs" role="tablist" aria-label="Campaign views" @keydown="selectTab">
            <button
              v-for="t in tabs"
              :id="`tab-${t.id}`"
              :key="t.id"
              type="button"
              role="tab"
              :aria-selected="tab === t.id"
              :tabindex="tab === t.id ? 0 : -1"
              aria-controls="cd-panel"
              @click="tab = t.id"
            >
              {{ t.label }}
            </button>
          </div>

          <section class="cd-stack" :class="{ 'is-off': campaign.joined && tab !== 'standings' }" aria-label="Your bib">
            <p v-if="feed.loading && !feed.leaderboard.length" class="quiet">Loading standings…</p>
            <BibStack
              v-else
              ref="stack"
              :entries="feed.leaderboard"
              :metric="campaign.metric"
              :campaign-name="campaign.name"
              :me-id="meId"
              :pooled="pooled"
              :pooled-value="feed.pooledProgressValue"
              :pooled-target="campaign.targetValue"
              :joined="campaign.joined"
              :joining="joining"
              :time-note="schedule.note"
            :last-active="lastActive"
              @join="join"
            />
            <p v-if="joinError" class="notice-inline" role="alert">{{ joinError }}</p>
            <p v-if="privateGoal && campaign.joined" class="private">
              <svg viewBox="0 0 20 20" aria-hidden="true"><rect x="4" y="9" width="12" height="8" rx="1.5" /><path d="M7 9V6.5a3 3 0 0 1 6 0V9" /></svg>
              <span><b>Private</b> · {{ privateGoal }}</span>
            </p>
          </section>

          <section v-if="campaign.joined" id="cd-panel" class="cd-panel" role="tabpanel" :aria-labelledby="`tab-${tab}`">
            <template v-if="tab === 'standings'">
              <p v-if="!feed.leaderboard.length && !feed.loading" class="quiet">Nobody has joined yet.</p>
              <ol v-else class="rows">
                <li v-for="e in feed.leaderboard" :key="e.userId" :class="{ me: e.userId === meId }">
                  <component
                    :is="pooled || !campaign.joined ? 'div' : 'button'"
                    :type="pooled || !campaign.joined ? undefined : 'button'"
                    class="row"
                    @click="stack?.focusUser(e.userId)"
                  >
                    <span class="chip">{{ e.rank }}</span>
                    <span class="who">
                      <b>{{ e.userId === meId ? "You" : e.userName }}</b>
                      <small v-if="e.targetValue > 0 && e.progressValue >= e.targetValue" class="finisher">Finisher</small>
                    <small v-else>{{ rowNote(e.userId, e.progressValue) }}</small>
                    </span>
                    <span class="val">
                      {{ fmt(e.progressValue) }}
                      <small v-if="pooled && feed.pooledProgressValue > 0">
                        {{ Math.round((e.progressValue / feed.pooledProgressValue) * 100) }}%
                      </small>
                    </span>
                  </component>
                </li>
              </ol>
            </template>

            <template v-else-if="tab === 'feed'">
              <p v-if="!feed.activities.length" class="quiet">
                No activities yet. Members' Strava uploads appear here as they sync.
              </p>
              <ol v-else class="rows">
                <li v-for="a in feed.activities" :key="a.id" class="activity">
                  <span class="who">
                    <b>{{ a.userId === meId ? "You" : a.userName }}</b>
                    <span class="act-name">{{ a.name }}</span>
                    <small>{{ a.type }} · {{ toDate(a.startDate) ? formatRelative(toDate(a.startDate)!) : "" }}</small>
                  </span>
                  <span class="val">{{ fmt(activityMetricValue(a, campaign.metric)) }}</span>
                </li>
              </ol>
            </template>

            <template v-else>
              <p v-if="!feed.timelineSeries.length" class="quiet">No progress to chart yet.</p>
              <TimelineChart
                v-else
                :series="feed.timelineSeries"
                :target-value="feed.timelineTargetValue"
                :me-id="meId"
                :format-value="fmt"
              />
            </template>
          </section>
        </div>

        <div v-if="campaign.joined" class="leave">
          <button v-if="!confirmLeave" type="button" class="pill pill--quiet" @click="confirmLeave = true">Leave campaign</button>
          <div v-else class="notice" role="group" aria-label="Confirm leaving">
            <p>Leave {{ campaign.name }}? Your activities stop counting here.</p>
            <span class="leave-actions">
              <button type="button" class="pill" @click="confirmLeave = false">Stay</button>
              <button type="button" class="pill pill--primary" :disabled="leaving" @click="leave">
                {{ leaving ? "Leaving…" : "Leave" }}
              </button>
            </span>
          </div>
          <p v-if="leaveError" class="notice-inline" role="alert">{{ leaveError }}</p>
        </div>

        <footer class="attribution">
          <img :src="poweredByStrava" alt="Powered by Strava" width="158" height="16" />
        </footer>
      </template>
    </div>
  </main>
</template>

<style scoped>
.cd-page {
  background: var(--paper);
  min-height: calc(100vh - 58px);
}
.cd {
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

.cd-head {
  margin: 0.75rem 0 1rem;
}
h1 {
  margin: 0;
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
  text-wrap: balance;
  overflow-wrap: anywhere;
}
.meta {
  margin: 0.4rem 0 0;
  color: var(--ink-muted);
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}
.desc {
  margin: 0.5rem 0 0;
  max-width: 65ch;
}

.cd-body {
  display: grid;
  grid-template-areas: "tabs" "stack" "panel";
}
.cd-tabs {
  grid-area: tabs;
  display: flex;
  gap: 1.5rem;
  border-bottom: 2px solid var(--hairline);
  margin-bottom: 1rem;
}
.cd-tabs button {
  border: 0;
  background: none;
  padding: 0.5rem 0 0.6rem;
  margin-bottom: -2px;
  border-bottom: 3px solid transparent;
  color: var(--ink-muted);
  font: 700 1rem var(--font-body);
  cursor: pointer;
}
.cd-tabs button[aria-selected="true"] {
  color: var(--ink);
  border-bottom-color: var(--tangerine);
}
.cd-stack {
  grid-area: stack;
}
.cd-panel {
  grid-area: panel;
  min-width: 0;
}

@media (max-width: 899px) {
  .cd-stack.is-off {
    display: none;
  }
  .cd-stack {
    margin-bottom: 1.25rem;
  }
}

@media (min-width: 900px) {
  .cd {
    padding: 1.5rem 1.5rem 3rem;
  }
  .cd-body {
    grid-template-columns: minmax(340px, 400px) minmax(0, 1fr);
    grid-template-areas: "stack tabs" "stack panel";
    grid-template-rows: auto 1fr;
    column-gap: 3rem;
    align-items: start;
  }
  .cd-stack {
    position: sticky;
    top: 1.5rem;
  }
}

.rows {
  list-style: none;
  margin: 0;
  padding: 0;
}
.rows li {
  border-bottom: 1px solid var(--hairline);
}
.row {
  display: grid;
  grid-template-columns: 2.25rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
  border: 0;
  background: none;
  padding: 0.7rem 0;
  color: var(--ink);
  font: inherit;
  text-align: left;
}
button.row {
  cursor: pointer;
}
button.row:hover .who b {
  text-decoration: underline 2px var(--cool-blue);
  text-underline-offset: 4px;
}
.chip {
  display: grid;
  place-items: center;
  height: 2rem;
  border: 2px solid var(--ink);
  border-radius: 6px;
  font: 400 0.9375rem var(--font-display);
  font-variant-numeric: tabular-nums;
}
.me .chip {
  background: var(--tangerine);
}
.who {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.who b,
.who small,
.act-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.who small {
  color: var(--ink-muted);
  font-size: 0.8125rem;
}
.who small.finisher {
  align-self: flex-start;
  margin-top: 0.15rem;
  padding: 0.05rem 0.4rem;
  border-radius: 4px;
  background: var(--cool-blue);
  color: var(--ink);
  font: 400 0.6875rem/1.4 var(--font-display);
  text-transform: uppercase;
}
.val {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.val small {
  color: var(--ink-muted);
  font-weight: 400;
  font-size: 0.8125rem;
}
.activity {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  padding: 0.7rem 0;
}

.private {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 1rem 0 0;
  padding: 0.6rem 0.75rem;
  border: 2px dashed var(--ink);
  border-radius: 10px;
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}
.private svg {
  width: 1.1rem;
  height: 1.1rem;
  flex: none;
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
  margin-top: 1rem;
  padding: 0.75rem 1rem;
  border: 2px solid var(--ink);
  border-radius: 10px;
  background: var(--cool-blue);
}
.notice p {
  margin: 0;
}
.notice-inline {
  margin: 1rem 0 0;
  font-weight: 700;
}
.leave {
  margin-top: 2.5rem;
}
.leave-actions {
  display: flex;
  gap: 0.5rem;
}
.attribution {
  margin-top: 2rem;
}
.attribution img {
  display: block;
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

.back:focus-visible,
.cd-tabs button:focus-visible,
.row:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: 2px;
}
</style>
