<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { LeaderboardEntry } from "../gen/fitter/v1/feed_pb";
import type { ActivityMetric } from "../gen/fitter/v1/common_pb";
import { formatMetricValue, ordinal } from "../lib/labels";
import { formatRelative } from "../lib/timestamp";

const props = defineProps<{
  entries: LeaderboardEntry[];
  metric: ActivityMetric;
  campaignName: string;
  meId: string | undefined;
  pooled: boolean;
  pooledValue: number;
  pooledTarget: number;
  joined: boolean;
  joining: boolean;
  timeNote: string;
  lastActive: Map<string, Date>;
}>();
const emit = defineEmits<{ join: [] }>();

const fmt = (v: number) => formatMetricValue(props.metric, v);

const myIndex = computed(() => props.entries.findIndex((e) => e.userId === props.meId));
// Stepping through the field only makes sense when the front bib is one person's standing.
const browsable = computed(() => !props.pooled && props.joined && myIndex.value !== -1);
const focus = ref(0);
const direction = ref<"ahead" | "behind">("ahead");
watch(myIndex, (i) => (focus.value = Math.max(i, 0)), { immediate: true });

const front = computed(() => (browsable.value ? props.entries[focus.value] : undefined));
// Outside browsing (pooled, or not joined) the peeks show the top two of the field.
const ahead = computed(() => (browsable.value ? props.entries[focus.value - 1] : props.entries[0]));
const behind = computed(() => (browsable.value ? props.entries[focus.value + 1] : props.entries[1]));
const me = computed(() => props.entries[myIndex.value]);

function step(to: "ahead" | "behind"): void {
  const next = focus.value + (to === "ahead" ? -1 : 1);
  if (!browsable.value || next < 0 || next >= props.entries.length) return;
  direction.value = to;
  focus.value = next;
}

function focusUser(userId: string): void {
  const i = props.entries.findIndex((e) => e.userId === userId);
  if (!browsable.value || i === -1 || i === focus.value) return;
  direction.value = i < focus.value ? "ahead" : "behind";
  focus.value = i;
}

function backToMe(): void {
  focusUser(props.meId ?? "");
}

defineExpose({ focusUser });

const isMe = (e: LeaderboardEntry | undefined) => !!e && e.userId === props.meId;
const finished = (e: LeaderboardEntry) => e.targetValue > 0 && e.progressValue >= e.targetValue;
const displayName = (e: LeaderboardEntry) => (isMe(e) ? "You" : e.userName);
// The tear-off tab is one short line; first names keep it there.
const shortName = (e: LeaderboardEntry) => (isMe(e) ? "you" : e.userName.split(" ")[0]);

const frontTab = computed(() => {
  const f = front.value;
  if (!f) return "";
  const a = ahead.value;
  if (a) {
    const gap = a.progressValue - f.progressValue;
    return gap > 0 ? `${fmt(gap)} to catch ${shortName(a)}` : `Level with ${shortName(a)}`;
  }
  const b = behind.value;
  if (b) return `${fmt(f.progressValue - b.progressValue)} clear of ${shortName(b)}`;
  if (!finished(f) && f.targetValue > 0) return `${fmt(f.targetValue - f.progressValue)} to finish`;
  return "";
});

const frontSub = computed(() => {
  const f = front.value;
  if (!f) return [];
  const left = finished(f) ? "Target reached" : `${fmt(f.targetValue - f.progressValue)} to go`;
  return props.timeNote ? [left, props.timeNote] : [left];
});

const pooledPct = computed(() =>
  props.pooledTarget > 0 ? Math.floor((props.pooledValue / props.pooledTarget) * 100) : 0,
);
const share = (e: LeaderboardEntry) =>
  props.pooledValue > 0 ? Math.round((e.progressValue / props.pooledValue) * 100) : 0;

function peekNote(e: LeaderboardEntry): string {
  const last = props.lastActive.get(e.userId);
  return last ? `${fmt(e.progressValue)} · ${formatRelative(last, true)}` : fmt(e.progressValue);
}

function peekBand(slot: "ahead" | "behind", e: LeaderboardEntry): [string, string] {
  if (!browsable.value) return [slot === "ahead" ? "Top" : "2nd", props.pooled ? `${share(e)}% of total` : fmt(e.progressValue)];
  const f = front.value!;
  const gap = Math.abs(e.progressValue - f.progressValue);
  return [slot === "ahead" ? "Ahead" : "Behind", `${slot === "ahead" ? "+" : "−"}${fmt(gap)}`];
}

const announcement = computed(() => {
  const f = front.value;
  if (!f) return "";
  return `${ordinal(f.rank)} of ${props.entries.length}, ${displayName(f)}, ${fmt(f.progressValue)}. ${frontTab.value}`;
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
    <div class="stage" :class="{ 'stage--solo': !front && !ahead }" role="group" aria-label="Your standing" @pointerdown="onPointerDown" @pointerup="onPointerUp">
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

      <Transition :name="`swap-${direction}`">
        <div v-if="front" :key="front.userId" class="bib bib--front">
          <span class="band" :class="{ 'band--blue': finished(front) }">
            <span>{{ finished(front) ? "Finisher" : campaignName }}</span><span>{{ displayName(front) }}</span>
          </span>
          <span class="bib-main">
            <span class="numeral" :class="{ 'numeral--long': String(front.rank).length > 1 }">{{ front.rank }}</span>
            <span class="bib-value">
              <b :class="{ 'value--long': fmt(front.progressValue).length > 9 }">{{ fmt(front.progressValue) }}</b>
              <small v-for="line in frontSub" :key="line">{{ line }}</small>
            </span>
          </span>
          <button v-if="ahead && frontTab" type="button" class="tab" @click="step('ahead')">
            <span>{{ frontTab }}</span>
            <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h11M11 5l5 5-5 5" /></svg>
          </button>
          <span v-else-if="frontTab" class="tab"><span>{{ frontTab }}</span></span>
        </div>

        <div v-else-if="joined && pooled" key="group" class="bib bib--front">
          <span class="band"><span>{{ campaignName }}</span><span>Group</span></span>
          <span class="bib-main">
            <span class="numeral numeral--long">{{ pooledPct }}%</span>
            <span class="bib-value">
              <b :class="{ 'value--long': fmt(pooledValue).length > 9 }">{{ fmt(pooledValue) }}</b>
              <small>of {{ fmt(pooledTarget) }}</small>
            </span>
          </span>
          <span v-if="me" class="tab"><span>You: {{ fmt(me.progressValue) }} · {{ share(me) }}% of the total</span></span>
        </div>

        <div v-else key="join" class="bib bib--front bib--join">
          <span class="band"><span>No. —</span><span>Unclaimed</span></span>
          <span class="join-body">
            <b>Get your bib</b>
            <small>Join to count your Strava activities and see the standings, feed and timeline.</small>
            <button type="button" class="btn-tangerine" :disabled="joining" @click="emit('join')">
              {{ joining ? "Joining…" : "Join campaign" }}
            </button>
          </span>
        </div>
      </Transition>
    </div>

    <p class="visually-hidden" aria-live="polite">{{ announcement }}</p>

    <div v-if="browsable && entries.length > 1" class="steps">
      <button type="button" class="pill" :disabled="focus === 0" @click="step('ahead')">
        <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M16 10H5M9 5l-5 5 5 5" /></svg>Ahead
      </button>
      <button v-if="focus !== myIndex" type="button" class="pill pill--quiet" @click="backToMe">Back to you</button>
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

.btn-tangerine {
  margin-top: 0.25rem;
  border: 2px solid var(--ink);
  border-radius: 999px;
  background: var(--tangerine);
  color: var(--ink);
  font: 700 0.9375rem var(--font-body);
  padding: 0.5rem 1.1rem;
  cursor: pointer;
}
.btn-tangerine:hover:not(:disabled) {
  background: var(--tangerine-hover);
}
.btn-tangerine:disabled {
  opacity: 0.6;
  cursor: progress;
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
.tab:focus-visible,
.btn-tangerine:focus-visible {
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
