<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { useSyncStore } from "../stores/sync";
import { useCampaignsStore } from "../stores/campaigns";
import { useCampaignFeedStore } from "../stores/campaignFeed";
import { LeaderboardWindow } from "../gen/fitter/v1/feed_pb";
import { useGoalsStore } from "../stores/goals";
import { useCatalogStore } from "../stores/catalog";
import { activityMetricValue } from "../lib/labels";
import { TOGETHER, lead, myEntries, primary, share, unitClause } from "../lib/standings";
import { toDate, formatRelative } from "../lib/timestamp";
import { plural } from "../lib/schedule";
import { detailParts, groupByDay, isNew, liftLines, pulse, recent } from "../lib/feed";
import BibStack from "../components/BibStack.vue";
import { ActivityMetric } from "../gen/fitter/v1/common_pb";
import { create } from "@bufbuild/protobuf";
import { LeaderboardEntrySchema, LeaderboardGroupSchema } from "../gen/fitter/v1/feed_pb";
import type { FeedActivity, LeaderboardEntry, LeaderboardGroup, TimelineSeries } from "../gen/fitter/v1/feed_pb";
import { errorMessage } from "../lib/errors";
import poweredByStrava from "../assets/strava/api_logo_pwrdBy_strava_horiz_black.svg";

// Async: the chart and ECharts load only when the Timeline tab first renders.
const TimelineChart = defineAsyncComponent(() => import("../components/charts/TimelineChart.vue"));

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

// Feed tab: last week's rows by day, what is new since the caller last looked, and the detail
// line they chose in Settings. The looked-at time moves on when they leave the tab.
const now = ref(new Date());
const recentRows = computed(() => recent(feed.value.activities, now.value.getTime()));
const newRows = computed(() => recentRows.value.filter((a) => isNew(a, feed.value.seenAt, meId.value)));
const feedDays = computed(() => groupByDay(recentRows.value, now.value));
const pulseText = computed(() => pulse(newRows.value));
const lastNew = computed(() => newRows.value.at(-1));
const details = (a: FeedActivity) => detailParts(a, auth.feedSettings).join(" · ");
const lifts = (a: FeedActivity) => (auth.feedSettings.showLifts ? liftLines(a.blocks) : []);
let feedViewed = false;
watch(tab, (now, was) => {
  if (now === "feed") feedViewed = true;
  else if (was === "feed") void feedStore.markSeen(props.id);
});
onBeforeUnmount(() => {
  if (tab.value === "feed" && feedViewed) void feedStore.markSeen(props.id);
});
const loadError = ref("");
const leaving = ref(false);
const confirmLeave = ref(false);
const leaveError = ref("");
const stack = ref<InstanceType<typeof BibStack> | null>(null);

// The bib stack can show one bib per goal (by sport group) or one per person.
const windows = [
  { id: LeaderboardWindow.WEEK, label: "Week" },
  { id: LeaderboardWindow.MONTH, label: "Month" },
  { id: LeaderboardWindow.CAMPAIGN, label: "Campaign" },
];
const windowError = ref("");
async function pickWindow(ev: Event): Promise<void> {
  const el = ev.target as HTMLSelectElement;
  // One board at a time: a second pick mid-load could land out of order, so it snaps back.
  if (feed.value.windowLoading) {
    el.value = String(feed.value.window);
    return;
  }
  const w = Number(el.value) as LeaderboardWindow;
  windowError.value = "";
  try {
    await feedStore.setWindow(props.id, w);
  } catch {
    windowError.value = "That board didn't load. Try again.";
  }
}

const bibMode = ref<"goal" | "person">("goal");
// The sport group on the bib (BibStack picks the default); the picker lives in the controls row.
const bibGroup = ref<string | null>(null);
const groupOptions = computed(() => [
  ...(pooled.value ? [{ key: TOGETHER, label: "Together" }] : []),
  ...bibGroups.value.map((g) => ({ key: g.sportGroup, label: g.sportGroupLabel })),
]);
const bibGroups = computed<LeaderboardGroup[]>(() => {
  if (bibMode.value === "goal" || !feed.value.people.length) return feed.value.leaderboardGroups;
  const entries = feed.value.people.map((p) =>
    create(LeaderboardEntrySchema, {
      userId: p.userId,
      userName: p.userName,
      userAvatarUrl: p.userAvatarUrl,
      goalId: `person:${p.userId}`,
      goalName: `${p.goalsSatisfied} of ${p.goalCount} ${p.goalCount === 1 ? "goal" : "goals"}`,
      progressRatio: p.averageRatio,
      satisfied: p.goalCount > 0 && p.goalsSatisfied === p.goalCount,
      rank: p.rank,
    }),
  );
  return [create(LeaderboardGroupSchema, { sportGroup: "", sportGroupLabel: "Everyone", entries })];
});
const stackEl = ref<HTMLElement | null>(null);

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
  if (!start || !end) return { meta: "", note: "", ended: false };
  const now = Date.now();
  if (now < start.getTime()) {
    const days = Math.ceil((start.getTime() - now) / DAY);
    return { meta: `Starts in ${days} ${days === 1 ? "day" : "days"}`, note: "", ended: false };
  }
  if (now > end.getTime()) return { meta: "Final standings", note: "", ended: true };
  const total = Math.max(1, Math.round((end.getTime() - start.getTime()) / DAY));
  const day = Math.min(total, Math.floor((now - start.getTime()) / DAY) + 1);
  const left = Math.ceil((end.getTime() - now) / DAY);
  return { meta: `Day ${day} of ${total}`, note: `${left} ${left === 1 ? "day" : "days"} left`, ended: false };
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
  // A session with none of the measure (a gym session against a distance goal) shows its time instead of "0 km".
  return !v ? fmt(ActivityMetric.MOVING_TIME, Number(a.movingTimeS)) : fmt(feedMetric.value, v);
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
  const next = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: tabs.length - 1 }[ev.key];
  if (next === undefined) return;
  ev.preventDefault();
  tab.value = tabs[(next + tabs.length) % tabs.length].id;
  (document.getElementById(`tab-${tab.value}`) as HTMLElement | null)?.focus();
}

// On a phone the stack sits above the list, so a tapped row would swap a bib you can't see.
// A keyboard activation (detail 0) keeps you where you are; the stack announces the change.
function showBib(ev: MouseEvent, goalId: string): void {
  stack.value?.focusGoal(goalId);
  const el = stackEl.value;
  if (!el || ev.detail === 0) return;
  const r = el.getBoundingClientRect();
  if (r.top >= 0 && r.bottom <= window.innerHeight) return;
  const calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  el.scrollIntoView({ block: "start", behavior: calm ? "auto" : "smooth" });
}

const goalsOf = (userId: string) =>
  feed.value.leaderboardGroups.flatMap((g) => g.entries).filter((e) => e.userId === userId);

function rowNote(e: LeaderboardEntry): string {
  const last = lastActive.value.get(e.userId);
  // The feed only carries recent activities, so progress without one in it is just older.
  if (last) return `active ${formatRelative(last)}`;
  return e.progressRatio > 0 ? "" : "no activity yet";
}

// Once the campaign is over, the top three ranks that actually scored get a Cool Blue chip.
const onPodium = (rank: number, scored: number) => schedule.value.ended && rank <= 3 && scored > 0;

// Members who added the same amount share a rank, like the goal standings.
const contributionRows = computed(() =>
  feed.value.pooledContributions.map((c, _i, all) => ({ ...c, rank: all.findIndex((x) => x.value === c.value) + 1 })),
);
function contributionNote(userId: string, value: number): string {
  const last = lastActive.value.get(userId);
  if (last) return `active ${formatRelative(last)}`;
  return value > 0 ? "" : "no activity yet";
}

// Units on the left already say how far; the share says how far along the goal that is.
const rowShare = (e: LeaderboardEntry) => (unitClause(e) ? `${share(e.progressRatio, 1)}%` : "");

// New Strava activities arrived: show them.
watch(() => useSyncStore().imported, load);

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

          <section ref="stackEl" class="cd-stack" :class="{ 'is-off': campaign.joined && tab !== 'standings' }" aria-label="Bib stack">
            <!-- One line of pickers: which group, which period, which view. -->
            <div v-if="campaign.joined && feed.loaded" class="bib-controls" :aria-busy="feed.windowLoading">
              <template v-if="groupOptions.length > 1">
                <label for="bib-group" class="visually-hidden">Group</label>
                <select id="bib-group" v-model="bibGroup" class="input">
                  <option v-for="o in groupOptions" :key="o.key" :value="o.key">{{ o.label }}</option>
                </select>
              </template>
              <label for="bib-window" class="visually-hidden">Period</label>
              <select id="bib-window" class="input" :value="feed.window" @change="pickWindow">
                <option v-for="w in windows" :key="w.id" :value="w.id">{{ w.label }}</option>
              </select>
              <template v-if="feed.people.length > 1">
                <label for="bib-view" class="visually-hidden">View</label>
                <select id="bib-view" v-model="bibMode" class="input">
                  <option value="goal">By goal</option>
                  <option value="person">By person</option>
                </select>
              </template>
            </div>
            <p v-if="windowError" class="field-error" role="alert">{{ windowError }}</p>
            <p v-if="feed.loading && !feed.loaded" class="quiet">Loading standings…</p>
            <template v-else>
            <BibStack
              :key="bibMode"
              ref="stack"
              v-model:group="bibGroup"
              :groups="bibGroups"
              :pooled="pooled"
              :contributions="feed.pooledContributions"
              :campaign-id="campaign.id"
              :campaign-name="campaign.name"
              :me-id="meId"
              :joined="campaign.joined"
              :time-note="schedule.note"
              :ended="schedule.ended"
              :last-active="lastActive"
            />
            </template>
            <!-- Without a goal and without a group target, the bib itself asks for one. -->
            <router-link v-if="campaign.joined && !schedule.ended && !mine.length && pooled" :to="`/goals/new?campaign=${campaign.id}`" class="pill add-goal">
              <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M10 4v12M4 10h12" /></svg>Set your goal
            </router-link>
            <p v-if="privateGoal && campaign.joined" class="private">
              <svg viewBox="0 0 20 20" aria-hidden="true"><rect x="4" y="9" width="12" height="8" rx="1.5" /><path d="M7 9V6.5a3 3 0 0 1 6 0V9" /></svg>
              <span><b>Private</b> · {{ privateGoal }}</span>
            </p>
          </section>

          <section v-if="campaign.joined" id="cd-panel" class="cd-panel" role="tabpanel" :aria-labelledby="`tab-${tab}`">
            <template v-if="tab === 'standings'">
              <section v-if="pooled && contributionRows.length" class="standings-group">
                <h2>Together</h2>
                <ol class="rows">
                  <li v-for="c in contributionRows" :key="c.userId" :class="{ me: c.userId === meId, podium: onPodium(c.rank, c.value) }">
                    <div class="row">
                      <span class="chip">{{ c.rank }}</span>
                      <span class="who">
                        <b>{{ c.userId === meId ? "You" : c.userName }}</b>
                        <small>{{ contributionNote(c.userId, c.value) }}</small>
                      </span>
                      <span class="val">
                        {{ fmt(pooled.metric, c.value) }}
                        <small>{{ share(c.value, pooled.value) }}%</small>
                      </span>
                    </div>
                  </li>
                </ol>
              </section>
              <p v-if="!feed.leaderboardGroups.length && !feed.loading" class="quiet">{{ feed.window === LeaderboardWindow.CAMPAIGN ? "No goals set yet. Standings appear once members set theirs." : "Nothing to score in this period yet." }}</p>
              <ol v-if="feed.people.length" class="rows standings-group">
                <li v-for="p in feed.people" :key="p.userId" :class="{ me: p.userId === meId, podium: onPodium(p.rank, p.averageRatio) }">
                  <div class="row">
                    <span class="chip">{{ p.rank }}</span>
                    <span class="who">
                      <b>{{ p.userId === meId ? "You" : p.userName }}</b>
                      <small>{{ p.goalsSatisfied }} of {{ p.goalCount }} {{ p.goalCount === 1 ? "goal" : "goals" }} done</small>
                      <small v-if="p.goalCount && p.goalsSatisfied === p.goalCount" class="finisher">Finisher</small>
                      <small v-if="p.earlyFinishes" class="finisher">Early finish</small>
                    </span>
                    <span class="val">{{ Math.floor(p.averageRatio * 100 + 1e-9) }}%<small>average</small></span>
                  </div>
                  <ul class="goal-rows">
                    <li v-for="e in goalsOf(p.userId)" :key="e.goalId">
                      <button type="button" class="row" :aria-current="stack?.frontGoalId === e.goalId ? 'true' : undefined" @click="showBib($event, e.goalId)">
                        <span />
                        <span class="who">
                          <b>{{ e.goalName }}</b>
                          <small>{{ rowNote(e) }}</small>
                        </span>
                        <span class="val">
                          {{ primary(e, catalog.format) }}
                          <small v-if="rowShare(e)">{{ rowShare(e) }}</small>
                        </span>
                      </button>
                    </li>
                  </ul>
                </li>
              </ol>
              <router-link v-if="mine.length && !schedule.ended" :to="`/goals/new?campaign=${campaign.id}`" class="pill add-goal">
                <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M10 4v12M4 10h12" /></svg>Add a goal
              </router-link>
            </template>

            <template v-else-if="tab === 'feed'">
              <p v-if="!recentRows.length" class="quiet">
                {{ feed.activities.length ? "Nothing in the last 7 days." : "No activities yet. Members' Strava uploads appear here as they sync." }}
              </p>
              <template v-else>
                <p v-if="pulseText" class="pulse">{{ pulseText }}</p>
                <p v-else-if="feed.seenAt" class="pulse">You're caught up.</p>
                <section v-for="g in feedDays" :key="g.label" class="day">
                  <h2>{{ g.label }}</h2>
                  <ol class="rows">
                    <template v-for="a in g.rows" :key="String(a.stravaActivityId)">
                      <li class="activity" :class="{ mine: a.userId === meId }">
                        <span class="who">
                          <b>{{ a.userId === meId ? "You" : a.userName }}</b>
                          <span class="act-name">{{ a.name }}</span>
                          <small>{{ a.sportType || a.type }} · {{ toDate(a.startDate) ? formatRelative(toDate(a.startDate)!) : "" }}<template v-if="a.source === 'manual'"> · logged manually</template></small>
                          <small v-if="details(a)">{{ details(a) }}</small>
                          <small v-for="line in lifts(a)" :key="line" class="lift-line">{{ line }}</small>
                        </span>
                        <span class="val">{{ activityValue(a) }}</span>
                      </li>
                      <li v-if="lastNew === a" class="tear" aria-hidden="true"><span>{{ newRows.length }} new since you looked</span></li>
                    </template>
                  </ol>
                </section>
                <p class="feed-foot"><router-link to="/settings">Choose which details show</router-link></p>
              </template>
            </template>

            <template v-else>
              <p v-if="!feed.timelineGroups.length && !feed.pooledTimeline?.points.length" class="quiet">No progress to chart yet.</p>
              <section v-if="feed.pooledTimeline?.points.length" class="chart">
                <h2>Together</h2>
                <TimelineChart
                  :series="[feed.pooledTimeline]"
                  :target-value="feed.pooledTimeline.targetValue"
                  :me-id="meId"
                  :format-value="(v) => fmt(feed.pooledTimeline!.metric, v)"
                />
              </section>
              <section v-for="g in feed.timelineGroups" :key="`${g.sportGroup}:${g.series[0]?.metric}`" class="chart">
                <h2>{{ g.sportGroupLabel }} · {{ catalog.metricLabel(g.series[0]?.metric ?? 0) }}</h2>
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
              <button type="button" class="pill pill--primary" v-busy="leaving" @click="leave">
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
.cd {
  max-width: 1120px;
  margin: 0 auto;
  padding: var(--space-5) var(--gutter) var(--space-8);
  font-family: var(--font-body);
  color: var(--ink);
}

.cd-head {
  margin: var(--space-4) 0 var(--space-5);
}
h1 {
  margin: 0;
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
  text-wrap: balance;
  overflow-wrap: anywhere;
}
.meta {
  margin: var(--space-2) 0 0;
  color: var(--ink-muted);
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}
.desc {
  margin: var(--space-3) 0 0;
  max-width: 65ch;
}

.cd-body {
  display: grid;
  grid-template-areas: "tabs" "stack" "panel";
}
.cd-tabs {
  grid-area: tabs;
  display: flex;
  gap: var(--space-7);
  border-bottom: 2px solid var(--hairline);
  margin-bottom: var(--space-5);
}
.cd-tabs button {
  min-width: 2.75rem;
  border: 0;
  background: none;
  padding: var(--space-3) 0 var(--row);
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
  scroll-margin-top: 1rem;
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
    margin-bottom: var(--space-6);
  }
}

@media (min-width: 900px) {
  .cd {
    padding: var(--space-7) var(--gutter) var(--space-9);
  }
  .cd-body {
    grid-template-columns: minmax(340px, 400px) minmax(0, 1fr);
    grid-template-areas: "stack tabs" "stack panel";
    grid-template-rows: auto 1fr;
    column-gap: var(--pane-gap);
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
  gap: var(--space-4);
  /* The inline padding gives the selected fill room without moving the columns. */
  width: calc(100% + 1rem);
  margin-inline: calc(var(--space-3) * -1);
  border: 0;
  border-radius: 6px;
  background: none;
  padding: var(--row) var(--space-3);
  color: var(--ink);
  font: inherit;
  text-align: left;
}
button.row {
  cursor: pointer;
}
/* The row on the bib in focus fills Cool Blue, like a checked choice. */
.row[aria-current="true"] {
  background: var(--cool-blue);
}
@media (hover: hover) {
  button.row:hover:not([aria-current="true"]) {
    background: var(--cool-blue-soft);
  }
}
.chip {
  display: grid;
  place-items: center;
  height: 2rem;
  border: 2px solid var(--line);
  border-radius: 6px;
  font: 400 0.9375rem var(--font-display);
  font-variant-numeric: tabular-nums;
}
.podium .chip {
  background: var(--cool-blue);
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
/* Compact selects sharing one row; each grows to its label, and the row wraps only if it must. */
.bib-controls {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-bottom: var(--space-5);
}
.bib-controls .input {
  flex: 1 1 auto;
  width: auto;
  min-width: 0;
  min-height: 2.5rem;
  padding: var(--space-1) 1.9rem var(--space-1) var(--space-3);
  background-position: right 0.5rem center;
  background-size: 1rem;
  font-weight: 700;
  font-size: 0.875rem;
}
@media (pointer: coarse) {
  .bib-controls .input {
    min-height: 2.75rem;
  }
}
.goal-rows {
  list-style: none;
  margin: 0 0 var(--space-3);
  padding: 0;
}
.goal-rows li {
  border-bottom: 0;
}
.goal-rows .row {
  padding-block: var(--space-2);
}
.who small.finisher {
  align-self: flex-start;
  margin-top: 0.15rem;
  margin-right: var(--space-1);
  padding: 0.05rem var(--space-2);
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
  gap: var(--space-4);
  padding: var(--row) 0 var(--row) var(--space-4);
  border-left: 4px solid transparent;
}
/* Yours, as everywhere: a Tangerine stroke, never text. */
.activity.mine {
  border-left-color: var(--tangerine);
}
.who small.lift-line {
  white-space: normal;
  overflow-wrap: anywhere;
}
.pulse {
  margin: 0 0 var(--space-4);
  color: var(--ink-muted);
  font-size: 0.875rem;
}
.day + .day {
  margin-top: var(--group-gap);
}
/* The tear-off motif again: a dashed rule with a Cool Blue tab, under the rows that are new. */
.rows li.tear {
  display: block;
  padding: 0;
  border-bottom: 0;
  border-top: 2px dashed var(--line);
  margin: var(--space-2) 0;
}
.tear span {
  display: inline-block;
  padding: var(--space-1) var(--space-3);
  border-radius: 0 0 6px 6px;
  background: var(--cool-blue);
  font: 400 0.75rem/1.3 var(--font-display);
  text-transform: uppercase;
}
.feed-foot {
  margin: var(--space-6) 0 0;
  font-size: 0.875rem;
}
.feed-foot a {
  display: inline-flex;
  align-items: center;
  min-height: 2.5rem;
  color: var(--ink-muted);
  font-weight: 600;
  text-decoration: underline;
  text-underline-offset: 3px;
}
@media (pointer: coarse) {
  .feed-foot a {
    min-height: 2.75rem;
  }
}

.add-goal {
  margin-top: var(--space-5);
}
.add-goal svg {
  width: 1rem;
  height: 1rem;
}
h2 {
  margin: 0 0 var(--space-1);
  font: 400 0.875rem/1.3 var(--font-display);
  text-transform: uppercase;
}
.standings-group + .standings-group,
.chart + .chart {
  margin-top: var(--group-gap);
}
.private {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin: var(--space-5) 0 0;
  padding: var(--space-3) var(--space-4);
  border: 2px dashed var(--line);
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
  padding: var(--space-5) 0;
}
.notice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  margin-top: var(--space-5);
  padding: var(--space-4) var(--space-5);
  border: 2px solid var(--line);
  border-radius: 10px;
  background: var(--cool-blue);
}
.notice p {
  margin: 0;
}
.notice-inline {
  margin: var(--space-5) 0 0;
  font-weight: 700;
}
.leave {
  margin-top: 2.5rem;
}
.leave-actions {
  display: flex;
  gap: var(--space-3);
}
.attribution {
  margin-top: var(--space-8);
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

.cd-tabs button:focus-visible,
.row:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: 2px;
}
</style>
