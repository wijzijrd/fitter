<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as echarts from "echarts";
import type { TimelineSeries } from "../../gen/fitter/v1/feed_pb";
import { toDate } from "../../lib/timestamp";

const props = defineProps<{
  series: TimelineSeries[];
  targetValue: number;
}>();

const el = ref<HTMLDivElement | null>(null);
let chart: echarts.ECharts | null = null;

function render(): void {
  if (!chart) return;
  const series = props.series.map((s) => ({
    name: s.label,
    type: "line",
    showSymbol: false,
    data: s.points.map((p) => [toDate(p.date)?.getTime() ?? 0, p.cumulativeValue]),
  }));

  chart.setOption({
    tooltip: { trigger: "axis" },
    legend: { show: props.series.length > 1 },
    grid: { left: 48, right: 24, top: 24, bottom: 32 },
    xAxis: { type: "time" },
    yAxis: { type: "value" },
    series: [
      ...series,
      {
        name: "Target",
        type: "line",
        data: [],
        markLine: {
          symbol: "none",
          silent: true,
          lineStyle: { type: "dashed", color: "#667066" },
          label: { formatter: "Target" },
          data: [{ yAxis: props.targetValue }],
        },
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

watch(() => [props.series, props.targetValue], render, { deep: true });
</script>

<template>
  <div ref="el" style="width: 100%; height: 320px" />
</template>
