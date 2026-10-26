<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as echarts from "echarts";
import type { TimelineSeries } from "../../gen/fitter/v1/feed_pb";
import { toDate } from "../../lib/timestamp";

const props = defineProps<{
  series: TimelineSeries[];
  // targetValue overrides the series' own targets (the pooled chart).
  targetValue?: number;
  meId: string | undefined;
  formatValue: (v: number) => string;
  labelFor?: (s: TimelineSeries) => string;
}>();

const el = ref<HTMLDivElement | null>(null);
let chart: echarts.ECharts | null = null;

// Mirrors the CSS tokens in style.css; ECharts draws on canvas and can't read them.
const INK = "#351e28";
const MUTED = "#5a4a52";
const TANGERINE = "#ff5c34";
const OTHERS = [INK, MUTED];
// Lines differ by dash as well as colour, and each is named at its end, so colour is never the only cue.
const DASHES = ["solid", "dashed", "dotted"] as const;
const FONT = "Hanken Grotesk Variable, system-ui, sans-serif";

const label = (s: TimelineSeries) => props.labelFor?.(s) ?? (s.userId && s.userId === props.meId ? "You" : s.label);
// One target rule only when every line chases the same number; mixed targets would mislead.
const target = computed(() => {
  if (props.targetValue !== undefined) return props.targetValue;
  const t = props.series[0]?.targetValue;
  return t && props.series.every((s) => s.targetValue === t) ? t : undefined;
});

// Full data for screen readers: cumulative value per member on each day that has a point.
const table = computed(() => {
  const days = new Map<number, Map<number, number>>();
  props.series.forEach((s, col) =>
    s.points.forEach((p) => {
      const t = toDate(p.date)?.getTime() ?? 0;
      if (!days.has(t)) days.set(t, new Map());
      days.get(t)!.set(col, p.value);
    }),
  );
  return [...days.entries()]
    .sort(([a], [b]) => a - b)
    .map(([t, cols]) => ({
      day: new Date(t).toLocaleDateString(undefined, { day: "numeric", month: "short" }),
      values: props.series.map((_, col) => (cols.has(col) ? props.formatValue(cols.get(col)!) : "")),
    }));
});

// Text alternative for the canvas: each line's latest cumulative value.
const summary = computed(() => {
  const parts = props.series.map((s) => {
    const last = s.points[s.points.length - 1];
    return `${label(s)} ${props.formatValue(last?.value ?? 0)}`;
  });
  const t = target.value === undefined ? "" : ` Target ${props.formatValue(target.value)}.`;
  return `Progress over time.${t} Latest: ${parts.join(", ")}.`;
});

function render(): void {
  if (!chart) return;
  let other = 0;
  // Ending the time axis at the last point keeps the target rule out of the end-label column.
  const lastT = Math.max(...props.series.flatMap((s) => s.points.map((p) => toDate(p.date)?.getTime() ?? 0)));
  const series = props.series.map((s) => {
    const mine = !!s.userId && s.userId === props.meId;
    const i = mine ? 0 : other++;
    return {
      name: label(s),
      type: "line",
      showSymbol: false,
      z: mine ? 3 : 2,
      lineStyle: {
        width: mine ? 4 : 2,
        color: mine ? TANGERINE : OTHERS[i % OTHERS.length],
        type: mine ? "solid" : DASHES[i % DASHES.length],
      },
      itemStyle: { color: mine ? TANGERINE : OTHERS[i % OTHERS.length] },
      endLabel: {
        show: true,
        formatter: "{a}",
        color: INK,
        fontFamily: FONT,
        fontWeight: 700,
        backgroundColor: "#ffffff",
        padding: [1, 3],
        distance: 8,
      },
      data: carryForward(s.points.map((p) => [toDate(p.date)?.getTime() ?? 0, p.value]), lastT),
    };
  });

  chart.setOption(
    {
      animation: !matchMedia("(prefers-reduced-motion: reduce)").matches,
      textStyle: { fontFamily: FONT, color: INK },
      tooltip: { trigger: "axis", valueFormatter: (v: number) => props.formatValue(v) },
      grid: { left: 56, right: 110, top: 24, bottom: 32 },
      xAxis: {
        type: "time",
        max: Number.isFinite(lastT) ? lastT : undefined,
        axisLine: { lineStyle: { color: INK } },
        axisLabel: { color: MUTED, formatter: "{d} {MMM}", hideOverlap: true },
      },
      yAxis: {
        type: "value",
        // Keep the target rule on the chart even while everyone is well short of it.
        max: target.value === undefined ? undefined : (v: { max: number }) => Math.max(v.max, target.value!),
        // Axis ticks are round numbers; drop the formatter's ".0".
        axisLabel: { color: MUTED, formatter: (v: number) => props.formatValue(v).replace(/\.0(?=\D|$)/, "") },
        splitLine: { lineStyle: { color: "#e9e2e6" } },
      },
      series: [
        ...series,
        ...(target.value === undefined ? [] : [{
          name: "Target",
          type: "line",
          // Below the member lines, so end labels print over the target rule, not under it.
          z: 1,
          data: [],
          markLine: {
            symbol: "none",
            silent: true,
            lineStyle: { type: "dashed", color: INK, width: 2 },
            label: { formatter: "Target", position: "insideStartTop", color: INK, fontFamily: FONT, fontWeight: 700 },
            data: [{ yAxis: target.value }],
          },
        }]),
      ],
    },
    true,
  );
  spreadEndLabels(series.map((s) => s.data[s.data.length - 1]));
}

// A cumulative total holds flat after someone's last activity; drawing that keeps every
// line (and its end label) running to the same last day.
function carryForward(data: number[][], lastT: number): number[][] {
  const last = data[data.length - 1];
  return last && Number.isFinite(lastT) && last[0] < lastT ? [...data, [lastT, last[1]]] : data;
}

// ECharts' label layout doesn't cover end labels, so friends with similar totals
// would print on top of each other; nudge them apart vertically instead.
const LABEL_GAP = 18;
function spreadEndLabels(lastPoints: (number[] | undefined)[]): void {
  const ys = lastPoints.map((p, i) => ({ i, y: p ? (chart!.convertToPixel({ seriesIndex: i }, p) as number[])[1] : NaN }));
  const placed = ys.filter((l) => !Number.isNaN(l.y)).sort((a, b) => a.y - b.y);
  const offsets = new Array(lastPoints.length).fill(0);
  let prev = -Infinity;
  for (const l of placed) {
    const y = Math.max(l.y, prev + LABEL_GAP);
    offsets[l.i] = y - l.y;
    prev = y;
  }
  chart!.setOption({ series: offsets.map((dy) => ({ endLabel: { offset: [0, dy] } })) });
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

watch(() => [props.series, target.value, props.meId], render, { deep: true });
</script>

<template>
  <div ref="el" role="img" :aria-label="summary" style="width: 100%; height: 320px" />
  <!-- A table ignores width and overflow, so the wrapper is what clips it; hiding the table itself still widens the page. -->
  <div class="visually-hidden">
    <table>
      <caption>Progress by day</caption>
      <thead>
        <tr>
          <th scope="col">Day</th>
          <th v-for="s in series" :key="s.clauseId || s.label" scope="col">{{ label(s) }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in table" :key="row.day">
          <th scope="row">{{ row.day }}</th>
          <td v-for="(v, i) in row.values" :key="i">{{ v }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
