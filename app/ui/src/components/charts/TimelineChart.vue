<script setup lang="ts">
// TimelineChart renders one line per TimelineSeries (a user/goal/clause, or
// the pooled series), each with a dashed target line drawn from that
// series' own target_value — series can now have different metrics
// (Task 7), so there is no longer a single shared target line.
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as echarts from "echarts";
import type { TimelineSeries } from "../../gen/fitter/v1/feed_pb";
import { toDate } from "../../lib/timestamp";
import { resolveThemeColor } from "../../lib/theme";

const props = defineProps<{ series: TimelineSeries[] }>();

const el = ref<HTMLDivElement | null>(null);
let chart: echarts.ECharts | null = null;

function render(): void {
  if (!chart) return;
  const targetColor = resolveThemeColor("--muted-foreground", "#667066");

  const series = props.series.map((s) => ({
    name: s.label,
    type: "line" as const,
    showSymbol: false,
    data: s.points.map((p) => [toDate(p.date)?.getTime() ?? 0, p.value]),
    markLine: {
      symbol: "none",
      silent: true,
      lineStyle: { type: "dashed" as const, color: targetColor },
      label: { formatter: `${s.label} target` },
      data: [{ yAxis: s.targetValue }],
    },
  }));

  chart.setOption({
    tooltip: { trigger: "axis" },
    legend: { show: props.series.length > 1 },
    grid: { left: 48, right: 24, top: 24, bottom: 32 },
    xAxis: { type: "time" },
    yAxis: { type: "value" },
    series,
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

watch(() => props.series, render, { deep: true });
</script>

<template>
  <div ref="el" style="width: 100%; height: 320px" />
</template>
