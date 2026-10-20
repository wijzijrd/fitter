<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { useCampaignsStore } from "../stores/campaigns";
import { useCampaignFeedStore } from "../stores/campaignFeed";
import { useGoalsStore } from "../stores/goals";
import { useCatalogStore } from "../stores/catalog";
import { activityMetricValue } from "../lib/labels";
import { lead, myEntries, primary, unitClause } from "../lib/standings";
import { toDate, formatRelative } from "../lib/timestamp";
import { plural } from "../lib/schedule";
import TimelineChart from "../components/charts/TimelineChart.vue";
import BibStack from "../components/BibStack.vue";
import { ActivityMetric } from "../gen/fitter/v1/common_pb";
import type { FeedActivity, LeaderboardEntry, TimelineSeries } from "../gen/fitter/v1/feed_pb";
import { errorMessage } from "../lib/errors";
import poweredByStrava from "../assets/strava/api_logo_pwrdBy_strava_horiz_black.svg";

const props = defineProps<{ id: string }>();
const router = useRouter();

const auth = useAuthStore();
const campaignsStore = useCampaignsStore();
const feedStore = useCampaignFeedStore();
const goalsStore = useGoalsStore();
const catalog = useCatalogStore();

type Tab = "standings" | "feed" | "timeline";
const tabs: { id: Tab; label: string }[] = [
  { id: "standings", label: "Standings" },
  { id: "feed", label: "Feed" },
  { id: "timeline", label: "Timeline" },
];
const tab = ref<Tab>("standings");
const loadError = ref("");
const leaving = ref(false);
const confirmLeave = ref(false);
const leaveError = ref("");
const stack = ref<InstanceType<typeof BibStack> | null>(null);

const campaign = computed(() => campaignsStore.campaigns.find((c) => c.id === props.id));
const feed = computed(() => feedStore.stateFor(props.id));
const meId = computed(() => auth.user?.id);
const fmt = catalog.format;

const pooled = computed(() => {
  const c = campaign.value;
  if (c?.metric === undefined || c.targetValue === undefined) return undefined;
  // The server leaves the total unset until an activity carries the metric.
  return { metric: c.metric, target: c.targetValue, value: feed.value.pooledProgressValue ?? 0 };
});
const mine = computed(() => myEntries(feed.value.leaderboardGroups, meId.value));

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

// The metrics this campaign is about for you: your goals' and the group target's.
const myMetrics = computed(() => {
  const set = new Set<ActivityMetric>(mine.value.flatMap((p) => p.entry.clauseDefs.map((c) => c.metric)));
  if (pooled.value) set.add(pooled.value.metric);
  return set;
});

// A private personal goal running now that measures something this campaign does.
const privateGoal = computed(() => {
  const now = new Date();
  const g = goalsStore.goals.find((g) => {
    const s = toDate(g.periodStart);
    const e = toDate(g.periodEnd);
    return s && e && s <= now && now <= e && g.clauses.some((c) => myMetrics.value.has(c.metric));
  });
  if (!g) return null;
  const pct = Math.floor((g.progress?.progressRatio ?? 0) * 100);
  const by = toDate(g.periodEnd)!.toLocaleDateString(undefined, { day: "numeric", month: "short" });
  return `${g.name} by ${by} · ${pct}%`;
});

// Feed rows show the measure you're racing on, falling back to time, which every activity has.
const feedMetric = computed(
  () => lead(mine.value)?.entry.clauseDefs[0]?.metric ?? pooled.value?.metric ?? ActivityMetric.DISTANCE,
);
function activityValue(a: FeedActivity): string {
  const v = activityMetricValue(a, feedMetric.value);
  return v === undefined ? fmt(ActivityMetric.MOVING_TIME, Number(a.movingTimeS)) : fmt(feedMetric.value, v);
}

const goalNames = computed(() => {
  const m = new Map<string, LeaderboardEntry>();
  for (const g of feed.value.leaderboardGroups) for (const e of g.entries) m.set(e.goalId, e);
  return m;
});
function seriesLabel(s: TimelineSeries, all: TimelineSeries[]): string {
  const who = s.userId === meId.value ? "You" : (goalNames.value.get(s.goalId)?.userName.split(" ")[0] ?? s.label);
  // Name the goal only when one person has two lines on the same chart.
  const twice = all.filter((x) => x.userId === s.userId).length > 1;
  return twice ? `${who} · ${goalNames.value.get(s.goalId)?.goalName ?? ""}` : who;
}

async function load(): Promise<void> {
  loadError.value = "";
  try {
    if (!auth.user) await auth.fetchCurrentUser();
    const [c] = await Promise.all([campaignsStore.getCampaign(props.id), catalog.ensureLoaded()]);
    // Standings, feed and timeline are members-only on the server.
    if (c.joined) await feedStore.fetchAll(props.id);
  } catch (err) {
    loadError.value = errorMessage(err, "We couldn't load this campaign. Check your connection and try again.");
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

function rowNote(e: LeaderboardEntry): string {
  const last = lastActive.value.get(e.userId);
  // The feed only carries recent activities, so progress without one in it is just older.
  if (last) return `${e.goalName} · active ${formatRelative(last)}`;
  return e.progressRatio > 0 ? e.goalName : `${e.goalName} · no activity yet`;
}

// Units on the left already say how far; the share says how far along the goal that is.
const rowShare = (e: LeaderboardEntry) => (unitClause(e) ? `${Math.floor(e.progressRatio * 100)}%` : "");

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
            {{ pooled ? `Group target ${fmt(pooled.metric, pooled.target)}` : "Own goals" }}
            <template v-if="schedule.meta"> · {{ schedule.meta }}</template>
            · {{ plural(campaign.memberCount, "member") }}
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
            <p v-if="feed.loading && !feed.loaded" class="quiet">Loading standings…</p>
            <BibStack
              v-else
              ref="stack"
              :groups="feed.leaderboardGroups"
              :pooled="pooled"
              :campaign-id="campaign.id"
              :campaign-name="campaign.name"
              :me-id="meId"
              :joined="campaign.joined"
              :time-note="schedule.note"
              :last-active="lastActive"
            />
            <!-- Without a goal and without a group target, the bib itself asks for one. -->
            <router-link v-if="campaign.joined && (mine.length || pooled)" :to="`/campaigns/${campaign.id}/join`" class="pill add-goal">
              <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M10 4v12M4 10h12" /></svg>{{ mine.length ? "Add a goal" : "Set your goal" }}
            </router-link>
            <p v-if="privateGoal && campaign.joined" class="private">
              <svg viewBox="0 0 20 20" aria-hidden="true"><rect x="4" y="9" width="12" height="8" rx="1.5" /><path d="M7 9V6.5a3 3 0 0 1 6 0V9" /></svg>
              <span><b>Private</b> · {{ privateGoal }}</span>
            </p>
          </section>

          <section v-if="campaign.joined" id="cd-panel" class="cd-panel" role="tabpanel" :aria-labelledby="`tab-${tab}`">
            <template v-if="tab === 'standings'">
              <div v-if="pooled" class="together">
                <h3>Together</h3>
                <p class="val">
                  {{ fmt(pooled.metric, pooled.value) }}
                  <small>of {{ fmt(pooled.metric, pooled.target) }} · {{ Math.floor((pooled.value / pooled.target) * 100) }}%</small>
                </p>
              </div>
              <p v-if="!feed.leaderboardGroups.length && !feed.loading" class="quiet">No goals set yet. Standings appear once members set theirs.</p>
              <section v-for="g in feed.leaderboardGroups" :key="g.sportGroup" class="standings-group">
                <h3 v-if="feed.leaderboardGroups.length > 1 || pooled">{{ g.sportGroupLabel }}</h3>
                <ol class="rows">
                  <li v-for="e in g.entries" :key="e.goalId" :class="{ me: e.userId === meId }">
                    <button type="button" class="row" @click="stack?.focusGoal(e.goalId)">
                      <span class="chip">{{ e.rank }}</span>
                      <span class="who">
                        <b>{{ e.userId === meId ? "You" : e.userName }}</b>
                        <small>{{ rowNote(e) }}</small>
                        <small v-if="e.satisfied" class="finisher">Finisher</small>
                      </span>
                      <span class="val">
                        {{ primary(e, catalog.format) }}
                        <small v-if="rowShare(e)">{{ rowShare(e) }}</small>
                      </span>
                    </button>
                  </li>
                </ol>
              </section>
            </template>

            <template v-else-if="tab === 'feed'">
              <p v-if="!feed.activities.length" class="quiet">
                No activities yet. Members' Strava uploads appear here as they sync.
              </p>
              <ol v-else class="rows">
                <li v-for="a in feed.activities" :key="String(a.stravaActivityId)" class="activity">
                  <span class="who">
                    <b>{{ a.userId === meId ? "You" : a.userName }}</b>
                    <span class="act-name">{{ a.name }}</span>
                    <small>{{ a.sportType || a.type }} · {{ toDate(a.startDate) ? formatRelative(toDate(a.startDate)!) : "" }}</small>
                  </span>
                  <span class="val">{{ activityValue(a) }}</span>
                </li>
              </ol>
            </template>

            <template v-else>
              <p v-if="!feed.timelineGroups.length && !feed.pooledTimeline?.points.length" class="quiet">No progress to chart yet.</p>
              <section v-if="feed.pooledTimeline?.points.length" class="chart">
                <h3>Together</h3>
                <TimelineChart
                  :series="[feed.pooledTimeline]"
                  :target-value="feed.pooledTimeline.targetValue"
                  :me-id="meId"
                  :format-value="(v) => fmt(feed.pooledTimeline!.metric, v)"
                />
              </section>
              <section v-for="g in feed.timelineGroups" :key="`${g.sportGroup}:${g.series[0]?.metric}`" class="chart">
                <h3>{{ g.sportGroupLabel }} · {{ catalog.metricLabel(g.series[0]?.metric ?? 0) }}</h3>
                <TimelineChart
                  :series="g.series"
                  :me-id="meId"
                  :format-value="(v) => fmt(g.series[0]?.metric ?? 0, v)"
                  :label-for="(s) => seriesLabel(s, g.series)"
                />
              </section>
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

.add-goal {
  margin-top: 1rem;
}
.add-goal svg {
  width: 1rem;
  height: 1rem;
}
h3 {
  margin: 0 0 0.25rem;
  font: 400 0.875rem/1.3 var(--font-display);
  text-transform: uppercase;
}
.standings-group + .standings-group,
.together + .standings-group,
.chart + .chart {
  margin-top: 1.75rem;
}
.together {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  padding-bottom: 0.7rem;
  border-bottom: 2px solid var(--ink);
}
.together .val {
  margin: 0;
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
