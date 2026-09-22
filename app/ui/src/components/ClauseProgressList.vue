<script setup lang="ts">
// ClauseProgressList renders one row per goal clause — metric label,
// current vs. target formatted per the metric's unit, satisfied state, and
// (when relevant) the "counted N of M" nullable-metric detail — replacing
// the old single aggregate progress bar throughout the app (goal cards,
// personal goal display, leaderboard entries).
import { computed, onMounted } from "vue";
import { useCatalogStore } from "@/stores/catalog";
import type { GoalClause, ClauseProgress } from "@/gen/fitter/v1/goal_pb";
import { comparatorLabels } from "@/lib/labels";
import { formatMetricValue } from "@/lib/metricFormat";
import { statusTier, badgeVariantForTier, progressIndicatorClassForTier } from "@/lib/statusTier";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";

const props = defineProps<{
  clauses: GoalClause[];
  progress?: ClauseProgress[];
}>();

const catalog = useCatalogStore();
onMounted(() => catalog.ensureLoaded());

interface Row {
  position: number;
  clause: GoalClause;
  progress?: ClauseProgress;
  tier: ReturnType<typeof statusTier>;
}

const rows = computed<Row[]>(() => {
  const byPosition = new Map<number, ClauseProgress>();
  for (const p of props.progress ?? []) byPosition.set(p.position, p);
  return [...props.clauses]
    .sort((a, b) => a.position - b.position)
    .map((clause) => {
      const progress = byPosition.get(clause.position);
      return {
        position: clause.position,
        clause,
        progress,
        tier: statusTier(progress?.satisfied ?? false, progress?.progressRatio ?? 0),
      };
    });
});
</script>

<template>
  <div class="space-y-2">
    <div v-for="row in rows" :key="row.position" class="space-y-1">
      <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
        <span>
          <span class="font-medium">{{ catalog.metricLabel(row.clause.metric) }}</span>
          <span class="text-muted-foreground">
            — {{ comparatorLabels[row.clause.comparator] }}
            {{ formatMetricValue(catalog.metricInfo(row.clause.metric)?.unit ?? 0, row.clause.value) }}
          </span>
        </span>
        <Badge :variant="badgeVariantForTier(row.tier)">
          {{ row.progress?.satisfied ? "Satisfied" : "Not yet" }}
        </Badge>
      </div>
      <Progress
        :model-value="Math.min(100, Math.max(0, (row.progress?.progressRatio ?? 0) * 100))"
        :indicator-class="progressIndicatorClassForTier(row.tier)"
      />
      <p class="text-xs text-muted-foreground">
        <template v-if="row.progress?.hasValue">
          {{ formatMetricValue(catalog.metricInfo(row.clause.metric)?.unit ?? 0, row.progress.currentValue) }}
          so far
        </template>
        <template v-else>No qualifying activity yet</template>
        <template
          v-if="row.progress && row.progress.countedActivityCount < row.progress.consideredActivityCount"
        >
          · counted {{ row.progress.countedActivityCount }} of
          {{ row.progress.consideredActivityCount }} activities (some don't report this metric)
        </template>
      </p>
    </div>
  </div>
</template>
