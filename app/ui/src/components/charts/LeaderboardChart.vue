<script setup lang="ts">
// LeaderboardChart renders one sport group's entries as a horizontal bar of
// overall progress ratio (0-100%) — a quick-scan supplement to the detailed
// per-clause breakdown rendered alongside it in CampaignDetailView.
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as echarts from "echarts";
import type { LeaderboardEntry } from "../../gen/fitter/v1/feed_pb";
import { resolveThemeColor } from "../../lib/theme";

const props = defineProps<{ entries: LeaderboardEntry[] }>();

const el = ref<HTMLDivElement | null>(null);
let chart: echarts.ECharts | null = null;

function render(): void {
  if (!chart) return;
  const sorted = [...props.entries].sort((a, b) => a.rank - b.rank).reverse();
  const primary = resolveThemeColor("--primary", "#fc4c02");
  chart.setOption({
    tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
    grid: { left: 120, right: 24, top: 16, bottom: 24 },
    xAxis: { type: "value", max: 100, axisLabel: { formatter: "{value}%" } },
    // Keyed by user + goal name, not just userName — a user with two goals
    // in the same sport group would otherwise produce duplicate category
    // axis entries that collide in the chart.
    yAxis: { type: "category", data: sorted.map((e) => `${e.userName} — ${e.goalName}`) },
    series: [
      {
        type: "bar",
        data: sorted.map((e) => Math.round(Math.min(1, e.progressRatio) * 100)),
        itemStyle: { color: primary },
        label: { show: true, position: "right", formatter: "{c}%" },
      },
    ],
  });
}

onMounted(() => {
  if (!el.value) return;
  chart = echarts.init(el.value);
  render();
  window.addEventListener("resize", handleResize);
});

function handleResize(): void {
  chart?.resize();
}

onBeforeUnmount(() => {
  window.removeEventListener("resize", handleResize);
  chart?.dispose();
});

watch(() => props.entries, render, { deep: true });
</script>

<template>
  <div ref="el" :style="{ width: '100%', height: Math.max(160, entries.length * 40) + 'px' }" />
</template>
