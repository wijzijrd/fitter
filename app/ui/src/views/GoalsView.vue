<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useGoalsStore } from "../stores/goals";
import { ActivityMetric } from "../gen/fitter/v1/common_pb";
import type { Goal } from "../gen/fitter/v1/goal_pb";
import { metricLabels, formatMetricValue, targetInputUnits } from "../lib/labels";
import { errorMessage } from "../lib/errors";

const store = useGoalsStore();
const showCreate = ref(false);
const creating = ref(false);
const createError = ref("");
const listError = ref("");
const deleting = ref("");
const deleteError = ref("");

const form = reactive({
  metric: ActivityMetric.DISTANCE,
  targetValue: 50,
  periodStart: new Date().toISOString().slice(0, 10),
  periodEnd: new Date(Date.now() + 30 * 86400_000).toISOString().slice(0, 10),
});
const targetUnit = computed(() => targetInputUnits[form.metric]);

async function load(): Promise<void> {
  listError.value = "";
  try {
    await store.fetchGoals();
  } catch (err) {
    listError.value = errorMessage(err, "Goals didn't load. Try again.");
  }
}

onMounted(load);

async function submitCreate(): Promise<void> {
  createError.value = "";
  if (form.periodEnd <= form.periodStart) {
    createError.value = "The end date needs to be after the start date.";
    return;
  }
  creating.value = true;
  try {
    await store.createGoal({
      metric: form.metric,
      targetValue: Number(form.targetValue) * targetUnit.value.factor,
      periodStart: new Date(form.periodStart),
      periodEnd: new Date(form.periodEnd),
    });
    showCreate.value = false;
  } catch (err) {
    createError.value = errorMessage(err, "The goal wasn't created. Try again.");
  } finally {
    creating.value = false;
  }
}

async function remove(g: Goal): Promise<void> {
  const what = `${metricLabels[g.metric]} goal of ${formatMetricValue(g.metric, g.targetValue)}`;
  if (!confirm(`Delete your ${what}? This can't be undone.`)) return;
  deleteError.value = "";
  deleting.value = g.id;
  try {
    await store.deleteGoal(g.id);
  } catch (err) {
    deleteError.value = errorMessage(err, "The goal wasn't deleted. Try again.");
  } finally {
    deleting.value = "";
  }
}
</script>

<template>
  <div class="container">
    <div style="display: flex; justify-content: space-between; align-items: center; gap: 1rem">
      <h1 class="page-title">My goals</h1>
      <button class="btn" @click="showCreate = !showCreate">{{ showCreate ? "Cancel" : "New goal" }}</button>
    </div>
    <p class="muted">Personal goals are private — only you can see them.</p>

    <form v-if="showCreate" class="card" @submit.prevent="submitCreate">
      <div class="field">
        <label for="goal-metric">Metric</label>
        <select id="goal-metric" v-model.number="form.metric">
          <option :value="ActivityMetric.DISTANCE">{{ metricLabels[ActivityMetric.DISTANCE] }}</option>
          <option :value="ActivityMetric.ELEVATION">{{ metricLabels[ActivityMetric.ELEVATION] }}</option>
          <option :value="ActivityMetric.TIME">{{ metricLabels[ActivityMetric.TIME] }}</option>
          <option :value="ActivityMetric.COUNT">{{ metricLabels[ActivityMetric.COUNT] }}</option>
        </select>
      </div>
      <label class="field">
        <span>Target ({{ targetUnit.unit }})</span>
        <input v-model.number="form.targetValue" type="number" min="1" step="any" required />
      </label>
      <label class="field">
        <span>Period start</span>
        <input v-model="form.periodStart" type="date" required />
      </label>
      <label class="field">
        <span>Period end</span>
        <input v-model="form.periodEnd" type="date" :min="form.periodStart" required />
      </label>
      <p v-if="createError" class="error" role="alert">{{ createError }}</p>
      <button class="btn" type="submit" :disabled="creating">{{ creating ? "Creating…" : "Create" }}</button>
    </form>

    <p v-if="listError" class="error" role="alert">
      {{ listError }} <button class="btn secondary" @click="load">Try again</button>
    </p>
    <p v-if="deleteError" class="error" role="alert">{{ deleteError }}</p>

    <div v-for="g in store.goals" :key="g.id" class="card">
      <strong>{{ metricLabels[g.metric] }}</strong>
      <div class="progress-bar" style="margin: 0.5rem 0">
        <span :style="{ width: Math.min(100, g.targetValue > 0 ? (g.progressValue / g.targetValue) * 100 : 0) + '%' }" />
      </div>
      <p class="muted">
        {{ formatMetricValue(g.metric, g.progressValue) }} / {{ formatMetricValue(g.metric, g.targetValue) }}
      </p>
      <button class="btn secondary" :disabled="deleting === g.id" @click="remove(g)">
        {{ deleting === g.id ? "Deleting…" : "Delete" }}
      </button>
    </div>
    <p v-if="!store.loading && !listError && store.goals.length === 0" class="muted">No goals yet.</p>
  </div>
</template>
