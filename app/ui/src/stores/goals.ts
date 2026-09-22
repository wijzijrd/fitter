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

  async function createGoal(input: GoalFormInput, campaignId?: string): Promise<Goal> {
    const resp = await goalClient.createGoal(toCreateGoalInit(input, campaignId));
    if (resp.goal) goals.value.unshift(resp.goal);
    return resp.goal!;
  }

  // updateGoal is a full replacement, not a patch — UpdateGoalRequest.clauses
  // doc comment.
  async function updateGoal(goalId: string, input: GoalFormInput): Promise<Goal> {
    const resp = await goalClient.updateGoal({
      goalId,
      name: input.name,
      sport: sportSelectorInit(input.sport),
      periodStart: input.periodStart ? fromDate(input.periodStart) : undefined,
      periodEnd: input.periodEnd ? fromDate(input.periodEnd) : undefined,
      clauses: input.clauses,
    });
    const idx = goals.value.findIndex((g) => g.id === goalId);
    if (idx !== -1 && resp.goal) goals.value[idx] = resp.goal;
    return resp.goal!;
  }

  async function deleteGoal(goalId: string): Promise<void> {
    await goalClient.deleteGoal({ goalId });
    goals.value = goals.value.filter((g) => g.id !== goalId);
  }

  return { goals, loading, fetchGoals, createGoal, updateGoal, deleteGoal };
});
