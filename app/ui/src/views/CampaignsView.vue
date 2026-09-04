<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { useCampaignsStore } from "../stores/campaigns";
import { ActivityMetric, CampaignMode } from "../gen/fitter/v1/common_pb";
import { metricLabels, modeLabels } from "../lib/labels";

const store = useCampaignsStore();
const showCreate = ref(false);

const form = reactive({
  name: "",
  description: "",
  mode: CampaignMode.POOLED,
  metric: ActivityMetric.DISTANCE,
  targetValue: 100,
  startDate: new Date().toISOString().slice(0, 10),
  endDate: new Date(Date.now() + 30 * 86400_000).toISOString().slice(0, 10),
});

onMounted(() => store.fetchCampaigns());

async function submitCreate(): Promise<void> {
  await store.createCampaign({
    name: form.name,
    description: form.description,
    mode: form.mode,
    metric: form.metric,
    targetValue: Number(form.targetValue),
    startDate: new Date(form.startDate),
    endDate: new Date(form.endDate),
  });
  showCreate.value = false;
  form.name = "";
  form.description = "";
}

async function join(campaignId: string): Promise<void> {
  await store.joinCampaign(campaignId);
}

async function leave(campaignId: string): Promise<void> {
  await store.leaveCampaign(campaignId);
}
</script>

<template>
  <div class="container">
    <div style="display: flex; justify-content: space-between; align-items: center">
      <h2>Campaigns</h2>
      <button class="btn" @click="showCreate = !showCreate">
        {{ showCreate ? "Cancel" : "New campaign" }}
      </button>
    </div>

    <form v-if="showCreate" class="card" @submit.prevent="submitCreate">
      <div class="field">
        <label>Name</label>
        <input v-model="form.name" required />
      </div>
      <div class="field">
        <label>Description</label>
        <textarea v-model="form.description" rows="2" />
      </div>
      <div class="field">
        <label>Mode</label>
        <select v-model.number="form.mode">
          <option :value="CampaignMode.POOLED">{{ modeLabels[CampaignMode.POOLED] }}</option>
          <option :value="CampaignMode.PER_MEMBER">{{ modeLabels[CampaignMode.PER_MEMBER] }}</option>
        </select>
      </div>
      <div class="field">
        <label>Metric</label>
        <select v-model.number="form.metric">
          <option :value="ActivityMetric.DISTANCE">{{ metricLabels[ActivityMetric.DISTANCE] }}</option>
          <option :value="ActivityMetric.ELEVATION">{{ metricLabels[ActivityMetric.ELEVATION] }}</option>
          <option :value="ActivityMetric.TIME">{{ metricLabels[ActivityMetric.TIME] }}</option>
          <option :value="ActivityMetric.COUNT">{{ metricLabels[ActivityMetric.COUNT] }}</option>
        </select>
      </div>
      <div class="field">
        <label>Target value (km, m, minutes, or count depending on metric)</label>
        <input v-model.number="form.targetValue" type="number" min="1" required />
      </div>
      <div class="field">
        <label>Start date</label>
        <input v-model="form.startDate" type="date" required />
      </div>
      <div class="field">
        <label>End date</label>
        <input v-model="form.endDate" type="date" required />
      </div>
      <button class="btn" type="submit">Create</button>
    </form>

    <p v-if="store.loading" class="muted">Loading…</p>
    <div v-for="c in store.campaigns" :key="c.id" class="card">
      <div style="display: flex; justify-content: space-between">
        <div>
          <router-link :to="`/campaigns/${c.id}`"><strong>{{ c.name }}</strong></router-link>
          <p class="muted">{{ modeLabels[c.mode] }} · {{ metricLabels[c.metric] }} · {{ c.memberCount }} members</p>
        </div>
        <button v-if="!c.joined" class="btn secondary" @click="join(c.id)">Join</button>
        <button v-else class="btn secondary" @click="leave(c.id)">Leave</button>
      </div>
    </div>
    <p v-if="!store.loading && store.campaigns.length === 0" class="muted">No campaigns yet — create one.</p>
  </div>
</template>
