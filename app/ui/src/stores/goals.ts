import { defineStore } from "pinia";
import { ref } from "vue";
import { goalClient } from "../lib/connect";
import type { Goal } from "../gen/fitter/v1/goal_pb";
import type { GoalFormInput } from "../lib/goalInput";
import { toCreateGoalInit } from "../lib/goalInput";
import { sportSelectorInit } from "../lib/sportSelector";
import { fromDate } from "../lib/timestamp";

export const useGoalsStore = defineStore("goals", () => {
  const goals = ref<Goal[]>([]);
  // Personal plus every campaign goal of the caller: the goal hub's list.
  const allGoals = ref<Goal[]>([]);
  const loading = ref(false);

  // fetchGoals lists personal goals (campaignId undefined) or, when
  // campaignId is set, only the CALLER'S OWN goals in that campaign — see
  // ListGoalsRequest.campaign_id ("WHERE user_id = ? AND campaign_id = ?"
  // server-side). It is NOT a campaign-wide shared view: for that, see
  // FeedService.GetCampaignLeaderboard, whose LeaderboardEntry now carries
  // every member's own clause_defs alongside their progress.
  async function fetchGoals(campaignId?: string): Promise<void> {
    loading.value = true;
    try {
      const resp = await goalClient.listGoals({ campaignId });
      goals.value = resp.goals;
    } finally {
      loading.value = false;
    }
  }

  async function fetchAllGoals(): Promise<void> {
    loading.value = true;
    try {
      allGoals.value = (await goalClient.listGoals({ all: true })).goals;
    } finally {
      loading.value = false;
    }
  }

  async function createGoal(input: GoalFormInput, campaignId?: string): Promise<Goal> {
    const resp = await goalClient.createGoal(toCreateGoalInit(input, campaignId));
    if (resp.goal) {
      if (!campaignId) goals.value.unshift(resp.goal);
      allGoals.value.unshift(resp.goal);
    }
    return resp.goal!;
  }

  // updateGoal is a full replacement, not a patch — UpdateGoalRequest.clauses
  // doc comment.
  // campaignId: "" makes the goal personal, an id moves it to that campaign.
  async function updateGoal(goalId: string, input: GoalFormInput, campaignId = ""): Promise<Goal> {
    const resp = await goalClient.updateGoal({
      goalId,
      name: input.name,
      sport: sportSelectorInit(input.sport),
      periodStart: input.periodStart ? fromDate(input.periodStart) : undefined,
      periodEnd: input.periodEnd ? fromDate(input.periodEnd) : undefined,
      clauses: input.clauses,
      recurrence: input.recurrence,
      campaignId,
    });
    const idx = goals.value.findIndex((g) => g.id === goalId);
    if (idx !== -1 && resp.goal) goals.value[idx] = resp.goal;
    const all = allGoals.value.findIndex((g) => g.id === goalId);
    if (all !== -1 && resp.goal) allGoals.value[all] = resp.goal;
    return resp.goal!;
  }

  async function deleteGoal(goalId: string): Promise<void> {
    await goalClient.deleteGoal({ goalId });
    goals.value = goals.value.filter((g) => g.id !== goalId);
    allGoals.value = allGoals.value.filter((g) => g.id !== goalId);
  }

  return { goals, allGoals, loading, fetchGoals, fetchAllGoals, createGoal, updateGoal, deleteGoal };
});
