import { defineStore } from "pinia";
import { ref } from "vue";
import { goalClient } from "../lib/connect";
import type { Goal } from "../gen/fitter/v1/goal_pb";
import type { ActivityMetric } from "../gen/fitter/v1/common_pb";
import { fromDate } from "../lib/timestamp";

export interface GoalInput {
  metric: ActivityMetric;
  targetValue: number;
  periodStart: Date;
  periodEnd: Date;
}

export const useGoalsStore = defineStore("goals", () => {
  const goals = ref<Goal[]>([]);
  const loading = ref(false);

  async function fetchGoals(): Promise<void> {
    loading.value = true;
    try {
      const resp = await goalClient.listGoals({});
      goals.value = resp.goals;
    } finally {
      loading.value = false;
    }
  }

  async function createGoal(input: GoalInput): Promise<void> {
    const resp = await goalClient.createGoal({
      metric: input.metric,
      targetValue: input.targetValue,
      periodStart: fromDate(input.periodStart),
      periodEnd: fromDate(input.periodEnd),
    });
    if (resp.goal) goals.value.unshift(resp.goal);
  }

  async function updateGoal(goalId: string, input: GoalInput): Promise<void> {
    const resp = await goalClient.updateGoal({
      goalId,
      targetValue: input.targetValue,
      periodStart: fromDate(input.periodStart),
      periodEnd: fromDate(input.periodEnd),
    });
    const idx = goals.value.findIndex((g) => g.id === goalId);
    if (idx !== -1 && resp.goal) goals.value[idx] = resp.goal;
  }

  async function deleteGoal(goalId: string): Promise<void> {
    await goalClient.deleteGoal({ goalId });
    goals.value = goals.value.filter((g) => g.id !== goalId);
  }

  return { goals, loading, fetchGoals, createGoal, updateGoal, deleteGoal };
});
