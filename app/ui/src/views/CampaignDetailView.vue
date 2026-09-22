<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useCampaignsStore } from "../stores/campaigns";
import { useCampaignFeedStore } from "../stores/campaignFeed";
import { useCatalogStore } from "../stores/catalog";
import { useGoalsStore } from "../stores/goals";
import { formatMetricValue } from "../lib/metricFormat";
import { toDate } from "../lib/timestamp";
import { allSports } from "../lib/sportSelector";
import { MetricUnit } from "../gen/fitter/v1/common_pb";
import type { GoalFormCore } from "../components/GoalForm.vue";
import GoalForm from "../components/GoalForm.vue";
import TimelineChart from "../components/charts/TimelineChart.vue";
import LeaderboardChart from "../components/charts/LeaderboardChart.vue";
import ClauseProgressList from "../components/ClauseProgressList.vue";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Alert, AlertDescription } from "@/components/ui/alert";

const props = defineProps<{ id: string }>();

const route = useRoute();
const router = useRouter();
const campaignsStore = useCampaignsStore();
const feedStore = useCampaignFeedStore();
const catalog = useCatalogStore();
const goalsStore = useGoalsStore();

const loadError = ref("");

const tab = computed<string>({
  get: () => (typeof route.query.tab === "string" ? route.query.tab : "feed"),
  set: (v) => router.replace({ query: { ...route.query, tab: v } }),
});

const campaign = computed(() => campaignsStore.campaigns.find((c) => c.id === props.id));
const feed = computed(() => feedStore.stateFor(props.id));

onMounted(async () => {
  await catalog.ensureLoaded();
  try {
    await Promise.all([campaignsStore.getCampaign(props.id), feedStore.fetchAll(props.id)]);
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : "Failed to load this campaign.";
  }
});

// --- add goal to this campaign ---------------------------------------------
// Period is intentionally not collected here — CreateGoalRequest forces the
// goal's period to the campaign window whenever campaign_id is set (see
// goal.proto), the same rule the join-campaign flow (CampaignsView.vue)
// relies on to omit period inputs entirely.

function blankGoalCore(): GoalFormCore {
  return { name: "", sport: allSports(), clauses: [] };
}

const showAddGoal = ref(false);
const addingGoal = ref(false);
const addGoalError = ref("");
const addGoalCore = ref<GoalFormCore>(blankGoalCore());
const addGoalCoreValid = ref(false);
const addGoalFormKey = ref(0);

function openAddGoal(): void {
  addGoalCore.value = blankGoalCore();
  addGoalCoreValid.value = false;
  addGoalError.value = "";
  addGoalFormKey.value += 1; // remounts GoalForm/ClauseBuilder for a fresh, blank instance
  showAddGoal.value = true;
}

async function submitAddGoal(): Promise<void> {
  if (!addGoalCoreValid.value) {
    addGoalError.value = "Add a name and at least one valid clause.";
    return;
  }
  addingGoal.value = true;
  addGoalError.value = "";
  try {
    await goalsStore.createGoal(
      {
        name: addGoalCore.value.name,
        sport: addGoalCore.value.sport,
        clauses: addGoalCore.value.clauses,
      },
      props.id,
    );
    showAddGoal.value = false;
    await feedStore.fetchAll(props.id); // refresh leaderboard/feed so the new goal shows up immediately
  } catch (e) {
    addGoalError.value = e instanceof Error ? e.message : "Failed to add goal.";
  } finally {
    addingGoal.value = false;
  }
}
</script>

<template>
  <div class="mx-auto max-w-4xl space-y-4 p-6">
    <Alert v-if="loadError" variant="destructive">
      <AlertDescription>{{ loadError }}</AlertDescription>
    </Alert>

    <template v-if="campaign">
      <router-link to="/" class="text-sm text-muted-foreground hover:underline">&larr; Campaigns</router-link>
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-xl font-semibold">{{ campaign.name }}</h2>
        <Dialog v-model:open="showAddGoal">
          <DialogTrigger as-child>
            <Button size="sm" @click="openAddGoal">Add goal</Button>
          </DialogTrigger>
          <DialogContent class="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle>Add a goal to {{ campaign.name }}</DialogTitle>
            </DialogHeader>
            <form :key="addGoalFormKey" class="space-y-4" @submit.prevent="submitAddGoal">
              <GoalForm
                v-model="addGoalCore"
                @update:valid="(v) => (addGoalCoreValid = v)"
              />
              <Alert v-if="addGoalError" variant="destructive">
                <AlertDescription>{{ addGoalError }}</AlertDescription>
              </Alert>
              <DialogFooter>
                <Button type="submit" :disabled="addingGoal">{{ addingGoal ? "Adding…" : "Add goal" }}</Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>
      <p class="text-sm text-muted-foreground">
        {{ toDate(campaign.startDate)?.toLocaleDateString() }} – {{ toDate(campaign.endDate)?.toLocaleDateString() }}
        · {{ campaign.memberCount }} members · {{ campaign.goalCount }} goals
      </p>
      <p v-if="campaign.description" class="text-sm">{{ campaign.description }}</p>
      <Badge v-if="campaign.metric !== undefined" variant="secondary">
        Pooled target: {{ catalog.metricLabel(campaign.metric) }}
        {{ formatMetricValue(catalog.metricInfo(campaign.metric)?.unit ?? 0, campaign.targetValue ?? 0) }}
      </Badge>

      <Tabs v-model="tab">
        <TabsList>
          <TabsTrigger value="feed">Feed</TabsTrigger>
          <TabsTrigger value="leaderboard">Leaderboard</TabsTrigger>
          <TabsTrigger value="timeline">Timeline</TabsTrigger>
        </TabsList>

        <p v-if="feed.loading" class="mt-4 text-sm text-muted-foreground">Loading…</p>

        <TabsContent value="feed" class="mt-4 space-y-3">
          <Card v-for="a in feed.activities" :key="a.stravaActivityId.toString()">
            <CardContent class="pt-4">
              <strong>{{ a.userName }}</strong> — {{ a.name }}
              <span class="text-muted-foreground">({{ a.sportType }})</span>
              <p class="mt-1 text-sm text-muted-foreground">
                {{ toDate(a.startDate)?.toLocaleDateString() }} ·
                {{ formatMetricValue(MetricUnit.METERS, a.distanceM) }} ·
                {{ formatMetricValue(MetricUnit.SECONDS, Number(a.movingTimeS)) }}
                <template v-if="a.elevationM > 0"> · {{ formatMetricValue(MetricUnit.METERS, a.elevationM) }} gain</template>
                <template v-if="a.averageHeartrate !== undefined">
                  · {{ formatMetricValue(MetricUnit.BPM, a.averageHeartrate) }} avg HR
                </template>
                <template v-if="a.averageWatts !== undefined">
                  · {{ formatMetricValue(MetricUnit.WATTS, a.averageWatts) }} avg
                </template>
              </p>
            </CardContent>
          </Card>
          <p v-if="!feed.loading && feed.activities.length === 0" class="text-sm text-muted-foreground">
            No activity yet.
          </p>
        </TabsContent>

        <TabsContent value="leaderboard" class="mt-4 space-y-4">
          <Alert v-if="feed.pooledMetric !== undefined">
            <AlertDescription>
              Pooled progress: {{ formatMetricValue(catalog.metricInfo(feed.pooledMetric)?.unit ?? 0, feed.pooledProgressValue ?? 0) }}
              / {{ formatMetricValue(catalog.metricInfo(feed.pooledMetric)?.unit ?? 0, feed.pooledTargetValue ?? 0) }}
            </AlertDescription>
          </Alert>

          <Card v-for="group in feed.leaderboardGroups" :key="group.sportGroup">
            <CardHeader>
              <CardTitle>{{ group.sportGroupLabel || "All sports" }}</CardTitle>
            </CardHeader>
            <CardContent class="space-y-4">
              <LeaderboardChart :entries="group.entries" />
              <div
                v-for="entry in [...group.entries].sort((a, b) => a.rank - b.rank)"
                :key="`${entry.userId}-${entry.goalId}`"
                class="space-y-2 rounded-lg border border-border p-3"
              >
                <div class="flex items-center justify-between">
                  <span class="font-medium">#{{ entry.rank }} {{ entry.userName }} — {{ entry.goalName }}</span>
                  <Badge :variant="entry.satisfied ? 'default' : 'outline'">
                    {{ entry.satisfied ? "Satisfied" : "In progress" }}
                  </Badge>
                </div>
                <ClauseProgressList :clauses="entry.clauseDefs" :progress="entry.clauses" />
              </div>
              <p v-if="group.entries.length === 0" class="text-sm text-muted-foreground">No entries yet.</p>
            </CardContent>
          </Card>
          <p v-if="feed.leaderboardGroups.length === 0" class="text-sm text-muted-foreground">
            No goals in this campaign yet.
          </p>
        </TabsContent>

        <TabsContent value="timeline" class="mt-4 space-y-6">
          <Card v-if="feed.pooledTimeline">
            <CardHeader>
              <CardTitle>Pooled</CardTitle>
            </CardHeader>
            <CardContent>
              <TimelineChart :series="[feed.pooledTimeline]" />
            </CardContent>
          </Card>
          <Card
            v-for="group in feed.timelineGroups"
            :key="`${group.sportGroup}-${group.series[0]?.metric ?? ''}`"
          >
            <CardHeader>
              <CardTitle>
                {{ group.sportGroupLabel || "All sports" }}
                <span v-if="group.series[0]" class="text-muted-foreground font-normal">
                  · {{ catalog.metricLabel(group.series[0].metric) }}
                </span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <TimelineChart :series="group.series" />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </template>
  </div>
</template>
