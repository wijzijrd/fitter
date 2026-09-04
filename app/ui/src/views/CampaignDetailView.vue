<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useCampaignsStore } from "../stores/campaigns";
import { useCampaignFeedStore } from "../stores/campaignFeed";
import { modeLabels, metricLabels, formatMetricValue, activityMetricValue } from "../lib/labels";
import { toDate } from "../lib/timestamp";
import TimelineChart from "../components/charts/TimelineChart.vue";
import LeaderboardChart from "../components/charts/LeaderboardChart.vue";
import { CampaignMode } from "../gen/fitter/v1/common_pb";

const props = defineProps<{ id: string }>();

const campaignsStore = useCampaignsStore();
const feedStore = useCampaignFeedStore();
const tab = ref<"feed" | "leaderboard" | "timeline">("feed");

const campaign = computed(() => campaignsStore.campaigns.find((c) => c.id === props.id));
const feed = computed(() => feedStore.stateFor(props.id));

onMounted(async () => {
  await campaignsStore.getCampaign(props.id);
  await feedStore.fetchAll(props.id);
});
</script>

<template>
  <div class="container" v-if="campaign">
    <router-link to="/">&larr; Campaigns</router-link>
    <h2>{{ campaign.name }}</h2>
    <p class="muted">
      {{ modeLabels[campaign.mode] }} · {{ metricLabels[campaign.metric] }} ·
      target {{ formatMetricValue(campaign.metric, campaign.targetValue) }} · {{ campaign.memberCount }} members
    </p>
    <p v-if="campaign.description">{{ campaign.description }}</p>

    <div class="card" style="display: flex; gap: 1rem">
      <button class="btn" :class="{ secondary: tab !== 'feed' }" @click="tab = 'feed'">Feed</button>
      <button class="btn" :class="{ secondary: tab !== 'leaderboard' }" @click="tab = 'leaderboard'">Leaderboard</button>
      <button class="btn" :class="{ secondary: tab !== 'timeline' }" @click="tab = 'timeline'">Timeline</button>
    </div>

    <div v-if="feed.loading" class="muted">Loading…</div>

    <div v-else-if="tab === 'feed'">
      <div v-for="a in feed.activities" :key="a.id" class="card">
        <strong>{{ a.userName }}</strong> — {{ a.name }} <span class="muted">({{ a.type }})</span>
        <p class="muted">
          {{ toDate(a.startDate)?.toLocaleDateString() }} ·
          {{ formatMetricValue(campaign.metric, activityMetricValue(a, campaign.metric)) }}
        </p>
      </div>
      <p v-if="feed.activities.length === 0" class="muted">No activity yet.</p>
    </div>

    <div v-else-if="tab === 'leaderboard'" class="card">
      <p v-if="campaign.mode === CampaignMode.POOLED" class="muted">
        Pooled progress: {{ formatMetricValue(campaign.metric, feed.pooledProgressValue) }} /
        {{ formatMetricValue(campaign.metric, campaign.targetValue) }}
      </p>
      <LeaderboardChart :entries="feed.leaderboard" />
      <table style="width: 100%; margin-top: 1rem">
        <thead>
          <tr><th style="text-align: left">#</th><th style="text-align: left">Member</th><th>Progress</th></tr>
        </thead>
        <tbody>
          <tr v-for="e in feed.leaderboard" :key="e.userId">
            <td>{{ e.rank }}</td>
            <td>{{ e.userName }}</td>
            <td>
              <div class="progress-bar"><span :style="{ width: Math.min(100, e.progressRatio * 100) + '%' }" /></div>
              <span class="muted">{{ formatMetricValue(campaign.metric, e.progressValue) }} / {{ formatMetricValue(campaign.metric, e.targetValue) }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="card">
      <TimelineChart :series="feed.timelineSeries" :target-value="feed.timelineTargetValue" />
    </div>
  </div>
</template>
