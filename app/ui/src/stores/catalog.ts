import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { catalogClient } from "../lib/connect";
import type { MetricInfo, SportGroupInfo } from "../gen/fitter/v1/catalog_pb";
import { ActivityMetric } from "../gen/fitter/v1/common_pb";

// useCatalogStore fetches the metric/sport taxonomy from CatalogService once
// and caches it for the lifetime of the app — every metric/sport dropdown
// and every unit-aware value formatter reads from here instead of a
// hand-maintained TypeScript map, since the server is the single source of
// truth for the 17-metric taxonomy (internal/metrics on the Go side).
export const useCatalogStore = defineStore("catalog", () => {
  const metrics = ref<MetricInfo[]>([]);
  const sportGroups = ref<SportGroupInfo[]>([]);
  const loaded = ref(false);
  const loading = ref(false);
  let inFlight: Promise<void> | null = null;

  const metricByEnum = computed(() => {
    const map = new Map<ActivityMetric, MetricInfo>();
    for (const m of metrics.value) map.set(m.metric, m);
    return map;
  });

  function metricInfo(metric: ActivityMetric): MetricInfo | undefined {
    return metricByEnum.value.get(metric);
  }

  function metricLabel(metric: ActivityMetric): string {
    return metricInfo(metric)?.label ?? "Unknown metric";
  }

  async function ensureLoaded(): Promise<void> {
    if (loaded.value) return;
    if (inFlight) return inFlight;
    loading.value = true;
    inFlight = (async () => {
      try {
        const [metricsResp, sportsResp] = await Promise.all([
          catalogClient.listMetrics({}),
          catalogClient.listSports({}),
        ]);
        metrics.value = metricsResp.metrics;
        sportGroups.value = sportsResp.groups;
        loaded.value = true;
      } finally {
        loading.value = false;
        inFlight = null;
      }
    })();
    return inFlight;
  }

  return {
    metrics,
    sportGroups,
    loaded,
    loading,
    metricInfo,
    metricLabel,
    ensureLoaded,
  };
});
