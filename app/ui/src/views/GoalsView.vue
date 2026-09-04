<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { useGoalsStore } from "../stores/goals";
import { ActivityMetric } from "../gen/fitter/v1/common_pb";
import { metricLabels, formatMetricValue } from "../lib/labels";

const store = useGoalsStore();
const showCreate = ref(false);

const form = reactive({
  metric: ActivityMetric.DISTANCE,
  targetValue: 50,
  periodStart: new Date().toISOString().slice(0, 10),
  periodEnd: new Date(Date.now() + 30 * 86400_000).toISOString().slice(0, 10),
});

onMounted(() => store.fetchGoals());

async function submitCreate(): Promise<void> {
  await store.createGoal({
    metric: form.metric,
    targetValue: Number(form.targetValue),
    periodStart: new Date(form.periodStart),
    periodEnd: new Date(form.periodEnd),
  });
  showCreate.value = false;
}
</script>

<template>
  <div class="container">
    <div style="display: flex; justify-content: space-between; align-items: center">
      <h2>My goals</h2>
      <button class="btn" @click="showCreate = !showCreate">{{ showCreate ? "Cancel" : "New goal" }}</button>
    </div>
    <p class="muted">Personal goals are private — only you can see them.</p>

    <form v-if="showCreate" class="card" @submit.prevent="submitCreate">
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
        <label>Target value</label>
        <input v-model.number="form.targetValue" type="number" min="1" required />
      </div>
      <div class="field">
        <label>Period start</label>
        <input v-model="form.periodStart" type="date" required />
      </div>
      <div class="field">
        <label>Period end</label>
        <input v-model="form.periodEnd" type="date" required />
      </div>
      <button class="btn" type="submit">Create</button>
    </form>

    <div v-for="g in store.goals" :key="g.id" class="card">
      <strong>{{ metricLabels[g.metric] }}</strong>
      <div class="progress-bar" style="margin: 0.5rem 0">
        <span :style="{ width: Math.min(100, (g.progressValue / g.targetValue) * 100) + '%' }" />
      </div>
      <p class="muted">
        {{ formatMetricValue(g.metric, g.progressValue) }} / {{ formatMetricValue(g.metric, g.targetValue) }}
      </p>
      <button class="btn secondary" @click="store.deleteGoal(g.id)">Delete</button>
    </div>
    <p v-if="store.goals.length === 0" class="muted">No goals yet.</p>
  </div>
</template>
