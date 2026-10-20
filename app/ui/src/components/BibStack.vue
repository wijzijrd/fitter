<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { LeaderboardEntry, LeaderboardGroup } from "../gen/fitter/v1/feed_pb";
import type { ActivityMetric } from "../gen/fitter/v1/common_pb";
import { useCatalogStore } from "../stores/catalog";
import { ordinal } from "../lib/labels";
import { amount, firstName, gap, lead, myEntries, primary } from "../lib/standings";
import { formatRelative } from "../lib/timestamp";

const props = defineProps<{
  groups: LeaderboardGroup[];
  // pooled is the campaign's group target, when it has one.
  pooled?: { metric: ActivityMetric; value: number; target: number };
  campaignId: string;
  campaignName: string;
  meId: string | undefined;
  joined: boolean;
  timeNote: string;
  lastActive: Map<string, Date>;
}>();

const catalog = useCatalogStore();
const fmt = catalog.format;
const TOGETHER = "__together__";

const mine = computed(() => myEntries(props.groups, props.meId));
const myLead = computed(() => lead(mine.value));
const options = computed(() => [
  ...(props.pooled ? [{ key: TOGETHER, label: "Together" }] : []),
  ...props.groups.map((g) => ({ key: g.sportGroup, label: g.sportGroupLabel })),
]);

const selected = ref("");
// Open on your best standing, else the group target, else the first group.
watch(
  options,
  () => {
    if (options.value.some((o) => o.key === selected.value)) return;
    selected.value = myLead.value?.group.sportGroup ?? options.value[0]?.key ?? "";
  },
  { immediate: true },
);

const group = computed(() => props.groups.find((g) => g.sportGroup === selected.value));
const entries = computed(() => group.value?.entries ?? []);
const together = computed(() => selected.value === TOGETHER);
// Your best entry in the selected group, if you have one there.
const myIndex = computed(() => entries.value.findIndex((e) => e.userId === props.meId));
const browsable = computed(() => props.joined && !together.value && !!mine.value.length && entries.value.length > 0);

const focus = ref(0);
const direction = ref<"ahead" | "behind">("ahead");
watch([myIndex, selected], () => (focus.value = Math.max(myIndex.value, 0)), { immediate: true });

const front = computed(() => (browsable.value ? entries.value[focus.value] : undefined));
// Outside browsing the peeks show the top two of the selected group.
const ahead = computed(() => (together.value ? undefined : browsable.value ? entries.value[focus.value - 1] : entries.value[0]));
const behind = computed(() => (together.value ? undefined : browsable.value ? entries.value[focus.value + 1] : entries.value[1]));

function step(to: "ahead" | "behind"): void {
  const next = focus.value + (to === "ahead" ? -1 : 1);
  if (!browsable.value || next < 0 || next >= entries.value.length) return;
  direction.value = to;
  focus.value = next;
}

function focusGoal(goalId: string): void {
  const g = props.groups.find((g) => g.entries.some((e) => e.goalId === goalId));
  if (!g) return;
  selected.value = g.sportGroup;
  const i = g.entries.findIndex((e) => e.goalId === goalId);
  if (!browsable.value || i === focus.value) return;
  direction.value = i < focus.value ? "ahead" : "behind";
  focus.value = i;
}

function backToMe(): void {
  if (myLead.value) focusGoal(myLead.value.entry.goalId);
}

defineExpose({ focusGoal });

const isMe = (e: LeaderboardEntry | undefined) => !!e && e.userId === props.meId;
const displayName = (e: LeaderboardEntry) => (isMe(e) ? "You" : e.userName);
// The tear-off tab is one short line; first names keep it there.
const shortName = (e: LeaderboardEntry) => (isMe(e) ? "you" : firstName(e));

const frontTab = computed(() => {
  const f = front.value;
  if (!f || !group.value) return "";
  const g = gap(group.value, f, fmt);
  switch (g.kind) {
    case "chase":
      return `${g.amount} to catch ${shortName(g.other)}`;
    case "level":
      return `Level with ${shortName(g.other)}`;
    case "clear":
      return `${g.amount} clear of ${shortName(g.other)}`;
    case "finish":
      return `${g.amount} to finish`;
    default:
      return "";
  }
});

const frontSub = computed(() => {
  const f = front.value;
  if (!f) return [];
  const left = f.satisfied ? "Goal reached" : `${amount(f, 1 - f.progressRatio, fmt)} to go`;
  return [f.goalName, props.timeNote ? `${left} · ${props.timeNote}` : left];
});

const pooledPct = computed(() => (props.pooled && props.pooled.target > 0 ? Math.floor((props.pooled.value / props.pooled.target) * 100) : 0));
const pooledLeft = computed(() => {
  const p = props.pooled;
  if (!p) return "";
  return p.value >= p.target ? "Target reached" : `${fmt(p.metric, p.target - p.value)} to go`;
});

function peekNote(e: LeaderboardEntry): string {
  const last = props.lastActive.get(e.userId);
  // Outside browsing the band already carries the value.
  const head = browsable.value ? primary(e, fmt) : e.goalName;
  return last ? `${head} · ${formatRelative(last, true)}` : browsable.value ? `${head} · ${e.goalName}` : head;
}

function peekBand(slot: "ahead" | "behind", e: LeaderboardEntry): [string, string] {
  if (!browsable.value) return [slot === "ahead" ? "Top" : "2nd", primary(e, fmt)];
  const f = front.value!;
  const d = Math.abs(e.progressRatio - f.progressRatio);
  return [slot === "ahead" ? "Ahead" : "Behind", `${slot === "ahead" ? "+" : "−"}${amount(f, d, fmt, e)}`];
}

const announcement = computed(() => {
  const f = front.value;
  if (together.value && props.pooled) return `Group at ${pooledPct.value}%, ${pooledLeft.value}`;
  if (!f) return "";
  return `${ordinal(f.rank)} of ${entries.value.length}, ${displayName(f)}, ${f.goalName}, ${primary(f, fmt)}. ${frontTab.value}`;
});

let startX = 0;
let startY = 0;
function onPointerDown(ev: PointerEvent): void {
  startX = ev.clientX;
  startY = ev.clientY;
}
function onPointerUp(ev: PointerEvent): void {
  const dx = ev.clientX - startX;
  const dy = ev.clientY - startY;
  if (Math.abs(dx) < 48 || Math.abs(dx) < Math.abs(dy)) return;
  step(dx > 0 ? "ahead" : "behind");
}
</script>

<template>
  <div class="bibs">
    <div v-if="joined && options.length > 1" class="switch" role="radiogroup" aria-label="Standings">
      <label v-for="o in options" :key="o.key" class="choice choice--pill">
        <input v-model="selected" type="radio" name="bib-group" :value="o.key" />
        <span>{{ o.label }}</span>
      </label>
    </div>

    <div class="stage" :class="{ 'stage--solo': !ahead && !behind }" role="group" aria-label="Your standing" @pointerdown="onPointerDown" @pointerup="onPointerUp">
      <component :is="browsable ? 'button' : 'div'" v-if="ahead" :type="browsable ? 'button' : undefined" class="peek peek--ahead" @click="step('ahead')">
        <span class="band band--blue"><span>{{ peekBand("ahead", ahead)[0] }}</span><span>{{ peekBand("ahead", ahead)[1] }}</span></span>
        <span class="peek-row">
          <span class="numeral numeral--peek">{{ ahead.rank }}</span>
          <span class="peek-who"><b>{{ displayName(ahead) }}</b><small>{{ peekNote(ahead) }}</small></span>
        </span>
      </component>
      <div v-else-if="browsable" class="peek peek--ahead peek--lead">
        <span class="band band--blue"><span>Leading</span><span></span></span>
        <span class="peek-row"><span class="peek-who"><b>Top of the field</b><small>Nobody ahead of {{ front && isMe(front) ? "you" : "them" }}</small></span></span>
      </div>

      <component :is="browsable ? 'button' : 'div'" v-if="behind" :type="browsable ? 'button' : undefined" class="peek peek--behind" @click="step('behind')">
        <span class="band band--blue"><span>{{ peekBand("behind", behind)[0] }}</span><span>{{ peekBand("behind", behind)[1] }}</span></span>
        <span class="peek-row">
          <span class="numeral numeral--peek">{{ behind.rank }}</span>
          <span class="peek-who"><b>{{ displayName(behind) }}</b><small>{{ peekNote(behind) }}</small></span>
        </span>
      </component>
      <div v-else-if="browsable && entries.length > 1" class="peek peek--behind peek--lead">
        <span class="band band--blue"><span>Last</span><span></span></span>
        <span class="peek-row"><span class="peek-who"><b>Back of the field</b><small>Nobody behind {{ front && isMe(front) ? "you" : "them" }}</small></span></span>
      </div>

      <Transition :name="`swap-${direction}`">
        <div v-if="front" :key="front.goalId" class="bib bib--front">
          <span class="band" :class="{ 'band--blue': front.satisfied }">
            <span>{{ front.satisfied ? "Finisher" : campaignName }}</span><span>{{ displayName(front) }}</span>
          </span>
          <span class="bib-main">
            <span class="numeral" :class="{ 'numeral--long': String(front.rank).length > 1 }">{{ front.rank }}</span>
            <span class="bib-value">
              <b :class="{ 'value--long': primary(front, fmt).length > 9 }">{{ primary(front, fmt) }}</b>
              <small v-for="line in frontSub" :key="line">{{ line }}</small>
            </span>
          </span>
          <button v-if="ahead && frontTab" type="button" class="tab" @click="step('ahead')">
            <span>{{ frontTab }}</span>
            <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h11M11 5l5 5-5 5" /></svg>
          </button>
          <span v-else-if="frontTab" class="tab"><span>{{ frontTab }}</span></span>
        </div>

        <div v-else-if="joined && together && pooled" key="group" class="bib bib--front">
          <span class="band" :class="{ 'band--blue': pooledPct >= 100 }"><span>{{ campaignName }}</span><span>Group</span></span>
          <span class="bib-main">
            <span class="numeral numeral--long">{{ pooledPct }}%</span>
            <span class="bib-value">
              <b :class="{ 'value--long': fmt(pooled.metric, pooled.value).length > 9 }">{{ fmt(pooled.metric, pooled.value) }}</b>
              <small>of {{ fmt(pooled.metric, pooled.target) }}</small>
              <small v-if="timeNote">{{ timeNote }}</small>
            </span>
          </span>
          <span class="tab"><span>{{ pooledLeft }} together</span></span>
        </div>

        <div v-else-if="joined" key="goal" class="bib bib--front bib--join">
          <span class="band"><span>{{ campaignName }}</span><span>You</span></span>
          <span class="join-body">
            <b>Set your goal</b>
            <small>You're in. Pick what you're chasing to take a place in the standings.</small>
            <router-link :to="`/campaigns/${campaignId}/join`" class="pill pill--primary">Set your goal</router-link>
          </span>
        </div>

        <div v-else key="join" class="bib bib--front bib--join">
          <span class="band"><span>No. —</span><span>Unclaimed</span></span>
          <span class="join-body">
            <b>Get your bib</b>
            <small>Join to count your Strava activities and see the standings, feed and timeline.</small>
            <router-link :to="`/campaigns/${campaignId}/join`" class="pill pill--primary">Join campaign</router-link>
          </span>
        </div>
      </Transition>
    </div>

    <p class="visually-hidden" aria-live="polite">{{ announcement }}</p>

    <div v-if="browsable && entries.length > 1" class="steps">
      <button type="button" class="pill" :disabled="focus === 0" @click="step('ahead')">
        <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M16 10H5M9 5l-5 5 5 5" /></svg>Ahead
      </button>
      <button v-if="myLead && front?.goalId !== myLead.entry.goalId" type="button" class="pill pill--quiet" @click="backToMe">Back to you</button>
      <button type="button" class="pill" :disabled="focus === entries.length - 1" @click="step('behind')">
        Behind<svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h11M11 5l5 5-5 5" /></svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.stage {
  position: relative;
  height: 356px;
  touch-action: pan-y;
}

.stage--solo {
  height: auto;
  padding-bottom: 6px;
}
.stage--solo .bib--front {
  position: relative;
  inset: auto;
}

.bib,
.peek {
  position: absolute;
  display: flex;
  flex-direction: column;
  border: 2px solid var(--ink);
  border-radius: 10px;
  overflow: hidden;
  background: var(--paper);
  color: var(--ink);
  text-align: left;
  font: inherit;
  padding: 0;
}

.peek {
  width: min(64%, 250px);
  transition: background-color 160ms var(--ease-out-expo);
}
button.peek {
  cursor: pointer;
}
button.peek:hover {
  background: var(--cool-blue-soft);
}
.peek--ahead {
  top: 0;
  left: 0;
}
.peek--behind {
  bottom: 0;
  right: 0;
}

.band {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.3rem 0.75rem;
  background: var(--tangerine);
  font: 400 0.75rem/1.3 var(--font-display);
  text-transform: uppercase;
  white-space: nowrap;
}
.band > span {
  overflow: hidden;
  text-overflow: ellipsis;
}
.band--blue {
  background: var(--cool-blue);
}

.peek-row {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.4rem 0.75rem 0.5rem;
  min-width: 0;
}
.peek-who {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.peek-who b,
.peek-who small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.peek-who small {
  color: var(--ink-muted);
  font-size: 0.8125rem;
}
.peek--lead .peek-row {
  padding-block: 0.75rem;
}

.numeral {
  font-family: var(--font-display);
  font-size: clamp(4.5rem, 24vw, 6.5rem);
  line-height: 0.85;
  font-variant-numeric: tabular-nums;
}
.numeral--long {
  font-size: clamp(3rem, 15vw, 4rem);
}
.numeral--peek {
  font-size: 2.75rem;
  line-height: 1;
}

.bib--front {
  top: 82px;
  left: 9%;
  right: 6%;
  min-height: 176px;
  z-index: 1;
  /* The one lift in the system: only the bib in focus sits on top of the stack. */
  box-shadow: 0 6px 0 var(--ink);
}

.bib-main {
  flex: 1;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.5rem 0.9rem 0.5rem;
}
.bib-value {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  text-align: right;
  min-width: 0;
}
.bib-value b {
  font: 400 1.625rem/1.1 var(--font-display);
  font-variant-numeric: tabular-nums;
}
.bib-value b.value--long {
  font-size: 1.125rem;
}
.bib-value small {
  color: var(--ink-muted);
  font-size: 0.8125rem;
}

.tab {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  width: 100%;
  border: 0;
  border-top: 2px dashed var(--ink);
  background: var(--cool-blue);
  color: var(--ink);
  padding: 0.5rem 0.9rem;
  font: 700 0.9375rem var(--font-body);
  text-align: left;
}
button.tab {
  cursor: pointer;
}
button.tab:hover {
  background: var(--cool-blue-deep);
}

.bib--join .join-body {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.4rem;
  padding: 0.75rem 0.9rem 0.9rem;
}
.join-body b {
  font: 400 1.5rem/1.1 var(--font-display);
}
.join-body small {
  color: var(--ink-muted);
  font-size: 0.875rem;
}

.join-body .pill {
  margin-top: 0.25rem;
}

.switch {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-bottom: 1rem;
}

.steps {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 0.6rem;
  padding: 1.25rem 0 0.25rem;
}

svg {
  width: 1.1rem;
  height: 1.1rem;
  flex: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.peek:focus-visible,
.tab:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: 2px;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}

.swap-ahead-enter-active,
.swap-behind-enter-active {
  transition: transform 380ms var(--ease-out-expo), opacity 220ms var(--ease-out-expo);
}
.swap-ahead-leave-active,
.swap-behind-leave-active {
  transition: opacity 140ms ease-out;
}
.swap-ahead-enter-from {
  transform: translate(-10%, -34%) scale(0.9);
  opacity: 0;
}
.swap-behind-enter-from {
  transform: translate(10%, 34%) scale(0.9);
  opacity: 0;
}
.swap-ahead-leave-to,
.swap-behind-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .swap-ahead-enter-active,
  .swap-behind-enter-active,
  .swap-ahead-leave-active,
  .swap-behind-leave-active {
    transition: none;
  }
}
</style>
