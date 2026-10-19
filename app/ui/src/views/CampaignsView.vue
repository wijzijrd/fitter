<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useAuthStore } from "../stores/auth";
import { useCampaignsStore } from "../stores/campaigns";
import { useCampaignFeedStore } from "../stores/campaignFeed";
import type { Campaign } from "../gen/fitter/v1/campaign_pb";
import type { LeaderboardEntry } from "../gen/fitter/v1/feed_pb";
import { CampaignMode } from "../gen/fitter/v1/common_pb";
import { metricLabels, formatMetricValue, ordinal } from "../lib/labels";
import { toDate } from "../lib/timestamp";
import { campaignPhase, daysUntil, plural, shortDate } from "../lib/schedule";
import { errorMessage } from "../lib/errors";
import CampaignBib from "../components/CampaignBib.vue";
import poweredByStrava from "../assets/strava/api_logo_pwrdBy_strava_horiz_black.svg";

const auth = useAuthStore();
const store = useCampaignsStore();
const feedStore = useCampaignFeedStore();

const listError = ref("");
const standing = reactive<Record<string, "loading" | "error" | "ok">>({});
const joining = reactive<Record<string, boolean>>({});
const joinError = reactive<Record<string, string>>({});

const meId = computed(() => auth.user?.id);
const byEnd = (a: Campaign, b: Campaign) => (toDate(a.endDate)?.getTime() ?? 0) - (toDate(b.endDate)?.getTime() ?? 0);
const byStart = (a: Campaign, b: Campaign) => (toDate(a.startDate)?.getTime() ?? 0) - (toDate(b.startDate)?.getTime() ?? 0);

const groups = computed(() => {
  const g = { racing: [] as Campaign[], soon: [] as Campaign[], finished: [] as Campaign[], open: [] as Campaign[] };
  for (const c of store.campaigns) {
    const phase = campaignPhase(c);
    if (!c.joined) {
      if (phase !== "finished") g.open.push(c);
    } else if (phase === "live") g.racing.push(c);
    else if (phase === "upcoming") g.soon.push(c);
    else g.finished.push(c);
  }
  g.racing.sort(byEnd);
  g.soon.sort(byStart);
  g.open.sort(byStart);
  g.finished.sort((a, b) => byEnd(b, a));
  return g;
});

async function loadStanding(id: string): Promise<void> {
  standing[id] = "loading";
  try {
    await feedStore.fetchLeaderboard(id);
    standing[id] = "ok";
  } catch {
    standing[id] = "error";
  }
}

async function load(): Promise<void> {
  listError.value = "";
  try {
    await Promise.all([store.fetchCampaigns(), auth.user ? null : auth.fetchCurrentUser()]);
  } catch (err) {
    listError.value = errorMessage(err, "Campaigns didn't load. Try again.");
    return;
  }
  // Standings are members-only on the server, and upcoming races have none yet.
  for (const c of store.campaigns) if (c.joined && campaignPhase(c) !== "upcoming") loadStanding(c.id);
}

onMounted(load);

const pooled = (c: Campaign) => c.mode === CampaignMode.POOLED;
const firstName = (e: LeaderboardEntry) => e.userName.split(" ")[0];

function rules(c: Campaign): string {
  return `${formatMetricValue(c.metric, c.targetValue)} ${pooled(c) ? "together" : "each"} · ${plural(c.memberCount, "member")}`;
}

function mine(c: Campaign) {
  const s = feedStore.stateFor(c.id);
  const i = s.leaderboard.findIndex((e) => e.userId === meId.value);
  return { entries: s.leaderboard, i, me: s.leaderboard[i] as LeaderboardEntry | undefined, total: s.pooledProgressValue };
}

interface Row {
  num: string;
  numLabel?: string;
  meta: string;
  gap: string;
  gapSub: string;
  finisher?: boolean;
  // closeness is the gap as a share of the target, so races in different units compare.
  closeness?: number;
  catchLine?: string;
}

function rowFor(c: Campaign): Row {
  const fmt = (v: number) => formatMetricValue(c.metric, v);
  const left = daysUntil(toDate(c.endDate));
  const when = left <= 1 ? "last day" : `${left} days left`;
  if (standing[c.id] !== "ok") return { num: "", meta: `${pooled(c) ? "Together" : metricLabels[c.metric]} · ${when}`, gap: "", gapSub: "" };

  const { entries, i, me, total } = mine(c);
  if (pooled(c)) {
    const pct = c.targetValue > 0 ? Math.floor((total / c.targetValue) * 100) : 0;
    const done = pct >= 100;
    return {
      num: `${pct}%`,
      numLabel: `Group at ${pct}%`,
      meta: `Together · ${when}${me ? ` · you ${Math.round(total > 0 ? (me.progressValue / total) * 100 : 0)}%` : ""}`,
      gap: done ? "Done" : fmt(c.targetValue - total),
      gapSub: done ? "target hit" : "to target",
      finisher: done,
    };
  }
  if (!me) return { num: "", meta: `of ${entries.length} · ${when}`, gap: "", gapSub: "No standing yet" };

  const done = me.targetValue > 0 && me.progressValue >= me.targetValue;
  const row: Row = { num: String(me.rank), numLabel: `${ordinal(me.rank)} of ${entries.length}`, meta: `of ${entries.length} · ${when}`, gap: "", gapSub: "", finisher: done };
  const ahead = entries[i - 1];
  const behind = entries[i + 1];
  if (ahead) {
    const gap = ahead.progressValue - me.progressValue;
    row.gap = gap > 0 ? fmt(gap) : "Level";
    row.gapSub = gap > 0 ? `to ${firstName(ahead)}` : `with ${firstName(ahead)}`;
    if (gap > 0 && me.targetValue > 0) {
      row.closeness = gap / me.targetValue;
      row.catchLine = `${fmt(gap)} to catch ${firstName(ahead)} in ${c.name}`;
    }
  } else if (behind) {
    row.gap = `+${fmt(me.progressValue - behind.progressValue)}`;
    row.gapSub = `on ${firstName(behind)}`;
  } else {
    row.gap = done ? "Done" : fmt(me.targetValue - me.progressValue);
    row.gapSub = done ? "target hit" : "to finish";
  }
  return row;
}

const racingRows = computed(() => groups.value.racing.map((c) => ({ c, row: rowFor(c) })));

const closestMove = computed(() => {
  const best = racingRows.value
    .filter((r) => r.row.closeness !== undefined)
    .sort((a, b) => a.row.closeness! - b.row.closeness!)[0];
  if (best) return { text: `Closest move: ${best.row.catchLine}`, to: `/campaigns/${best.c.id}` };
  const ready = racingRows.value.filter((r) => r.row.num);
  return ready.length && ready.length === racingRows.value.length ? { text: "Nobody to catch: you lead every race you're in.", to: "" } : null;
});

function finalResult(c: Campaign): { num: string; numLabel?: string; value: string; finisher: boolean } {
  if (standing[c.id] !== "ok") return { num: "", value: standing[c.id] === "error" ? "Didn't load" : "", finisher: false };
  const fmt = (v: number) => formatMetricValue(c.metric, v);
  const { entries, me, total } = mine(c);
  if (pooled(c)) {
    const pct = c.targetValue > 0 ? Math.floor((total / c.targetValue) * 100) : 0;
    return { num: `${pct}%`, numLabel: `Group reached ${pct}%`, value: fmt(total), finisher: pct >= 100 };
  }
  if (!me) return { num: "", value: "", finisher: false };
  return {
    num: String(me.rank),
    numLabel: `Finished ${ordinal(me.rank)} of ${entries.length}`,
    value: fmt(me.progressValue),
    finisher: me.targetValue > 0 && me.progressValue >= me.targetValue,
  };
}

async function join(c: Campaign): Promise<void> {
  joining[c.id] = true;
  joinError[c.id] = "";
  try {
    await store.joinCampaign(c.id);
    if (campaignPhase(c) === "live") loadStanding(c.id);
  } catch (err) {
    joinError[c.id] = errorMessage(err, "Joining didn't work. Try again.");
  } finally {
    joining[c.id] = false;
  }
}
</script>

<template>
  <main class="cl-page">
    <div class="cl">
      <header class="cl-head">
        <h1>Campaigns</h1>
        <router-link v-if="store.campaigns.length" to="/campaigns/new" class="pill pill--primary">New campaign</router-link>
      </header>

      <div v-if="listError" class="notice" role="alert">
        <p>{{ listError }}</p>
        <button type="button" class="pill" @click="load">Try again</button>
      </div>
      <p v-else-if="store.loading && !store.campaigns.length" class="quiet" aria-live="polite">Loading campaigns…</p>

      <CampaignBib v-else-if="!store.campaigns.length" :band="['No. —', 'Unclaimed']">
        <div class="unclaimed">
          <b>Not racing yet</b>
          <small>Start a campaign and invite the crew. Their Strava activities count from day one.</small>
          <router-link to="/campaigns/new" class="pill pill--primary">New campaign</router-link>
        </div>
      </CampaignBib>

      <div v-else class="cl-body">
        <section class="sheet" aria-labelledby="h-racing">
          <h2 id="h-racing" class="band"><span>Start list · Racing</span><span>{{ plural(groups.racing.length, "race") }}</span></h2>
          <template v-if="groups.racing.length">
            <div class="cols" aria-hidden="true"><span>No.</span><span>Race</span><span>Next gap</span></div>
            <ol class="rows">
              <li v-for="{ c, row } in racingRows" :key="c.id" class="row">
                <span class="no" :class="{ 'no--long': row.num.length > 2, pending: standing[c.id] === 'loading' }" aria-hidden="true">
                  {{ standing[c.id] === "loading" ? "–" : row.num }}
                </span>
                <span class="race">
                  <router-link :to="`/campaigns/${c.id}`" class="race-link">{{ c.name }}</router-link>
                  <small>
                    <span v-if="row.numLabel" class="visually-hidden">{{ row.numLabel }}, </span>{{ row.meta }}
                  </small>
                  <small v-if="row.finisher" class="finisher">Finisher</small>
                </span>
                <span v-if="standing[c.id] === 'error'" class="gap">
                  <button type="button" class="retry" @click="loadStanding(c.id)">Try again</button>
                  <small>Didn't load</small>
                </span>
                <span v-else class="gap">{{ row.gap }}<small>{{ row.gapSub }}</small></span>
              </li>
            </ol>
            <router-link v-if="closestMove?.to" :to="closestMove.to" class="tear">
              <span>{{ closestMove.text }}</span>
              <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h11M11 5l5 5-5 5" /></svg>
            </router-link>
            <p v-else-if="closestMove" class="tear">{{ closestMove.text }}</p>
          </template>
          <div v-else class="empty-row">
            <p>You're not in a race right now. Join one below or start your own.</p>
            <router-link to="/campaigns/new" class="pill">New campaign</router-link>
          </div>
        </section>

        <div class="side">
          <section v-if="groups.soon.length || groups.open.length" class="sheet" aria-labelledby="h-start">
            <h2 id="h-start" class="band band--blue"><span>On the start line</span><span>{{ groups.soon.length + groups.open.length }}</span></h2>
            <ol class="rows">
              <li v-for="c in groups.soon" :key="c.id" class="row">
                <span class="no no--long" aria-hidden="true">{{ daysUntil(toDate(c.startDate)) }}D</span>
                <span class="race">
                  <router-link :to="`/campaigns/${c.id}`" class="race-link">{{ c.name }}</router-link>
                  <small>
                    <span class="visually-hidden">Starts in {{ plural(daysUntil(toDate(c.startDate)), "day") }}. </span>You're in · starts {{ shortDate(toDate(c.startDate)) }}
                  </small>
                </span>
                <span class="gap"><small>{{ formatMetricValue(c.metric, c.targetValue) }} {{ pooled(c) ? "together" : "each" }}</small></span>
              </li>
              <li v-for="c in groups.open" :key="c.id" class="row">
                <span class="no" aria-hidden="true"></span>
                <span class="race">
                  <router-link :to="`/campaigns/${c.id}`" class="race-link">{{ c.name }}</router-link>
                  <small>{{ rules(c) }}</small>
                  <small v-if="joinError[c.id]" class="row-error" role="alert">{{ joinError[c.id] }}</small>
                </span>
                <span class="gap">
                  <button v-if="pooled(c)" type="button" class="pill" :disabled="joining[c.id]" @click="join(c)">
                    {{ joining[c.id] ? "Joining…" : "Join" }}
                  </button>
                  <router-link v-else :to="`/campaigns/${c.id}/join`" class="pill">Join</router-link>
                </span>
              </li>
            </ol>
          </section>

          <section v-if="groups.finished.length" class="sheet" aria-labelledby="h-results">
            <h2 id="h-results" class="band band--blue"><span>Results</span><span>{{ groups.finished.length }}</span></h2>
            <ol class="rows">
              <li v-for="c in groups.finished" :key="c.id" class="row">
                <span class="no" :class="{ 'no--long': finalResult(c).num.length > 2 }" aria-hidden="true">{{ finalResult(c).num }}</span>
                <span class="race">
                  <router-link :to="`/campaigns/${c.id}`" class="race-link">{{ c.name }}</router-link>
                  <small>
                    <span v-if="finalResult(c).numLabel" class="visually-hidden">{{ finalResult(c).numLabel }}, </span>Ended {{ shortDate(toDate(c.endDate)) }} · {{ metricLabels[c.metric] }}
                  </small>
                  <small v-if="finalResult(c).finisher" class="finisher">Finisher</small>
                </span>
                <span class="gap"><small>{{ finalResult(c).value }}</small></span>
              </li>
            </ol>
          </section>
        </div>
      </div>

      <footer class="attribution">
        <img :src="poweredByStrava" alt="Powered by Strava" width="158" height="16" />
      </footer>
    </div>
  </main>
</template>

<style scoped>
.cl-page {
  background: var(--paper);
  min-height: calc(100vh - 58px);
}
.cl {
  max-width: 1120px;
  margin: 0 auto;
  padding: 1rem 1rem 2rem;
  font-family: var(--font-body);
  color: var(--ink);
}
.cl-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin: 0.5rem 0 1.25rem;
}
.cl-head .pill {
  white-space: nowrap;
}
h1 {
  margin: 0;
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}

.sheet {
  border: 2px solid var(--ink);
  border-radius: 10px;
  overflow: hidden;
}
.sheet + .sheet {
  margin-top: 1.25rem;
}
.band {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  margin: 0;
  padding: 0.3rem 0.75rem;
  background: var(--tangerine);
  font: 400 0.75rem/1.3 var(--font-display);
  text-transform: uppercase;
  white-space: nowrap;
}
.band--blue {
  background: var(--cool-blue);
}

.cols,
.row {
  display: grid;
  grid-template-columns: 3.5rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  padding-inline: 0.75rem;
}
.cols {
  padding-block: 0.5rem 0.4rem;
  border-bottom: 2px solid var(--ink);
  color: var(--ink-muted);
  font: 400 0.75rem/1.2 var(--font-display);
  text-transform: uppercase;
}
.cols span:last-child {
  text-align: right;
}
.rows {
  list-style: none;
  margin: 0;
  padding: 0;
}
.row {
  position: relative;
  padding-block: 0.7rem;
  border-bottom: 1px solid var(--hairline);
  transition: background-color 160ms var(--ease-out-expo);
}
.rows li:last-child {
  border-bottom: 0;
}
.row:has(.race-link:hover) {
  background: var(--cool-blue-soft);
}

.no {
  font: 400 2.5rem/0.9 var(--font-display);
  font-variant-numeric: tabular-nums;
  text-align: center;
}
.no--long {
  font-size: 1.5rem;
}
.no.pending {
  color: var(--hairline);
}

.race {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.race-link {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  overflow: hidden;
  overflow-wrap: anywhere;
  color: var(--ink);
  font-weight: 700;
  font-size: 0.9375rem;
}
/* Stretched link: the race name carries the accessible name, the row carries the hit area. */
.race-link::after {
  content: "";
  position: absolute;
  inset: 0;
}
.race-link:focus-visible {
  outline: none;
}
.race-link:focus-visible::after {
  outline: 3px solid var(--ink);
  outline-offset: -3px;
}
.race small {
  color: var(--ink-muted);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
}
.race small.row-error {
  color: var(--ink);
  font-weight: 700;
}
.race small.finisher {
  align-self: flex-start;
  margin-top: 0.2rem;
  padding: 0.05rem 0.4rem;
  border-radius: 4px;
  background: var(--cool-blue);
  color: var(--ink);
  font: 400 0.6875rem/1.4 var(--font-display);
  text-transform: uppercase;
}

.gap {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  font-weight: 700;
  font-size: 0.9375rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  text-align: right;
}
.gap small {
  color: var(--ink-muted);
  font-weight: 400;
  font-size: 0.8125rem;
}
/* Controls in a row sit above its stretched link. */
.gap .pill,
.gap .retry {
  position: relative;
  z-index: 1;
  text-decoration: none;
}
.retry {
  border: 0;
  background: none;
  color: var(--ink);
  font: 700 0.9375rem var(--font-body);
  text-decoration: underline 2px !important;
  text-underline-offset: 4px;
  cursor: pointer;
  padding: 0;
}
.retry:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: 2px;
}

.tear {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin: 0;
  color: var(--ink);
  text-wrap: pretty;
  border-top: 2px dashed var(--ink);
  background: var(--cool-blue);
  padding: 0.55rem 0.75rem;
  font-weight: 700;
  font-size: 0.9375rem;
  font-variant-numeric: tabular-nums;
}
a.tear:hover {
  background: var(--cool-blue-deep);
}
a.tear:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: -3px;
}
.tear svg {
  width: 1.1rem;
  height: 1.1rem;
  flex: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.empty-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.9rem 0.75rem;
}
.empty-row p {
  margin: 0;
  max-width: 40ch;
}
.empty-row .pill {
  text-decoration: none;
}

.unclaimed {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.4rem;
  padding-bottom: 0.35rem;
}
.unclaimed b {
  font: 400 1.5rem/1.1 var(--font-display);
}
.unclaimed small {
  max-width: 40ch;
  color: var(--ink-muted);
  font-size: 0.875rem;
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
  padding: 0.75rem 1rem;
  border: 2px solid var(--ink);
  border-radius: 10px;
  background: var(--cool-blue);
}
.notice p {
  margin: 0;
}
.attribution {
  margin-top: 2.5rem;
}
.attribution img {
  display: block;
}

.side {
  margin-top: 1.25rem;
}

@media (min-width: 900px) {
  .cl {
    padding: 1.5rem 1.5rem 3rem;
  }
  .cl-body {
    display: grid;
    grid-template-columns: minmax(0, 640px) minmax(280px, 400px);
    column-gap: 2.5rem;
    align-items: start;
  }
  .side {
    margin-top: 0;
  }
}
</style>
