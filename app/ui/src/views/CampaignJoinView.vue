<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useCampaignsStore } from "../stores/campaigns";
import { CampaignMode } from "../gen/fitter/v1/common_pb";
import { metricLabels, targetInputUnits, formatMetricValue } from "../lib/labels";
import { toDate } from "../lib/timestamp";
import { dateRange, plural } from "../lib/schedule";
import { errorMessage } from "../lib/errors";
import CampaignBib from "../components/CampaignBib.vue";

const props = defineProps<{ id: string }>();
const router = useRouter();
const store = useCampaignsStore();

const loadError = ref("");
const joinError = ref("");
const targetError = ref("");
const joining = ref(false);
const target = ref<number | "">("");

const campaign = computed(() => store.campaigns.find((c) => c.id === props.id));
const unit = computed(() => targetInputUnits[campaign.value?.metric ?? 0]);
const perMember = computed(() => campaign.value?.mode === CampaignMode.PER_MEMBER);
const fmt = (v: number) => formatMetricValue(campaign.value!.metric, v);
const rawTarget = computed(() => (Number(target.value) || 0) * unit.value.factor);

async function load(): Promise<void> {
  loadError.value = "";
  try {
    const c = await store.getCampaign(props.id);
    if (c.joined) return void router.replace(`/campaigns/${c.id}`);
    target.value = Math.round((c.targetValue / targetInputUnits[c.metric].factor) * 10) / 10;
  } catch (err) {
    loadError.value = errorMessage(err, "We couldn't load this campaign. Try again.");
  }
}

onMounted(load);

async function join(): Promise<void> {
  const c = campaign.value!;
  targetError.value = perMember.value && !(rawTarget.value > 0) ? "Set a target above zero." : "";
  if (targetError.value) return;
  joining.value = true;
  joinError.value = "";
  try {
    // Only send an override when it differs, so a later change to the campaign target still applies.
    const own = perMember.value && rawTarget.value !== c.targetValue ? rawTarget.value : undefined;
    await store.joinCampaign(c.id, own);
    router.push(`/campaigns/${c.id}`);
  } catch (err) {
    joinError.value = errorMessage(err, "Joining didn't work. Try again.");
  } finally {
    joining.value = false;
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
        <h1>Get your bib</h1>
        <p class="meta">
          {{ campaign.name }} · {{ metricLabels[campaign.metric] }} ·
          {{ dateRange(toDate(campaign.startDate), toDate(campaign.endDate)) }} · {{ plural(campaign.memberCount, "member") }}
        </p>

        <form class="cj-body" novalidate @submit.prevent="join">
          <div class="preview" aria-hidden="true">
            <CampaignBib
              :band="[campaign.name, 'You']"
              :tab="perMember ? `Your target: ${fmt(rawTarget)}` : `Group target: ${fmt(campaign.targetValue)}`"
            >
              <span class="claim"><b>No. —</b><small>Assigned when you join</small></span>
              <span class="claim-value">
                <b>{{ fmt(0) }}</b>
                <small>{{ perMember ? `${fmt(rawTarget)} to go` : "Counts toward the group" }}</small>
              </span>
            </CampaignBib>
          </div>

          <div>
            <div v-if="perMember" class="rb-field">
              <label for="cj-target">Your target ({{ unit.unit }})</label>
              <input
                id="cj-target"
                v-model.number="target"
                class="input input--target"
                type="number"
                min="1"
                step="any"
                inputmode="decimal"
                :aria-invalid="!!targetError"
                aria-describedby="cj-target-hint"
              />
              <small id="cj-target-hint" :class="{ 'field-error': targetError }">
                {{ targetError || `The campaign target is ${fmt(campaign.targetValue)}. Change it to race your own line; standings still rank by total.` }}
              </small>
            </div>
            <p v-else class="lede">
              Everyone's activities add up to {{ fmt(campaign.targetValue) }}. Your Strava activities count from the moment you join.
            </p>

            <p v-if="joinError" class="notice" role="alert">{{ joinError }}</p>
            <button type="submit" class="pill pill--primary" :disabled="joining">{{ joining ? "Joining…" : "Join campaign" }}</button>
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
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  text-align: right;
}
.claim-value b {
  font: 400 1.625rem/1.1 var(--font-display);
  font-variant-numeric: tabular-nums;
}
.input--target {
  max-width: 12rem;
  font: 400 1.625rem/1.1 var(--font-display);
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
