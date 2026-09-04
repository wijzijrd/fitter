<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as echarts from "echarts";
import type { LeaderboardEntry } from "../../gen/fitter/v1/feed_pb";

const props = defineProps<{ entries: LeaderboardEntry[] }>();

const el = ref<HTMLDivElement | null>(null);
let chart: echarts.ECharts | null = null;

function render(): void {
  if (!chart) return;
  const sorted = [...props.entries].sort((a, b) => a.rank - b.rank).reverse();
  chart.setOption({
    tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
    grid: { left: 120, right: 24, top: 16, bottom: 24 },
    xAxis: { type: "value" },
    yAxis: { type: "category", data: sorted.map((e) => e.userName) },
    series: [
      {
        type: "bar",
        data: sorted.map((e) => e.progressValue),
        itemStyle: { color: "#fc4c02" },
        label: { show: true, position: "right" },
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
