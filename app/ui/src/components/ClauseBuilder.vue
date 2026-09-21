<script setup lang="ts">
// ClauseBuilder is the repeating goal-clause sub-form used by both the
// personal-goal form and the join-campaign flow. It edits a list of
// ClauseInput rows (scope + metric + comparator + unit-aware value) and
// keeps two things in sync with the parent:
//   - `update:modelValue`: the best-effort parsed clause list (invalid rows
//     are included with value 0 so the row count the parent sees always
//     matches what's on screen)
//   - `update:valid`: whether every row currently parses and satisfies its
//     metric's scope support, so the parent can gate submission
import { computed, onMounted, reactive, watch } from "vue";
import { z } from "zod";
import { useCatalogStore } from "@/stores/catalog";
import { ActivityMetric, ClauseScope, Comparator } from "@/gen/fitter/v1/common_pb";
import type { ClauseInput } from "@/lib/clauses";
import { scopeLabels, comparatorLabels } from "@/lib/labels";
import { baseValueToEditString, editStringToBaseValue, editUnitHint } from "@/lib/metricFormat";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Plus, X } from "@lucide/vue";

const props = defineProps<{ modelValue: ClauseInput[] }>();
const emit = defineEmits<{
  (e: "update:modelValue", value: ClauseInput[]): void;
  (e: "update:valid", valid: boolean): void;
}>();

const catalog = useCatalogStore();

interface ClauseRow {
  key: number;
  scope: ClauseScope;
  metric: ActivityMetric;
  comparator: Comparator;
  valueText: string;
}

let nextKey = 0;
const rows = reactive<ClauseRow[]>([]);
let initialized = false;

function defaultMetric(): ActivityMetric {
  return catalog.metrics[0]?.metric ?? ActivityMetric.DISTANCE;
}

function rowFromInput(c: ClauseInput): ClauseRow {
  const unit = catalog.metricInfo(c.metric)?.unit;
  return {
    key: nextKey++,
    scope: c.scope,
    metric: c.metric,
    comparator: c.comparator,
    valueText: unit !== undefined ? baseValueToEditString(unit, c.value) : String(c.value),
  };
}

function blankRow(): ClauseRow {
  const metric = defaultMetric();
  const supportsAgg = catalog.metricInfo(metric)?.supportsAggregateScope ?? true;
  return {
    key: nextKey++,
    scope: supportsAgg ? ClauseScope.AGGREGATE : ClauseScope.SINGLE,
    metric,
    comparator: Comparator.GTE,
    valueText: "",
  };
}

onMounted(async () => {
  await catalog.ensureLoaded();
  if (initialized) return;
  initialized = true;
  if (props.modelValue.length > 0) {
    rows.push(...props.modelValue.map(rowFromInput));
  } else {
    rows.push(blankRow());
  }
});

function addRow(): void {
  rows.push(blankRow());
}

function removeRow(key: number): void {
  const idx = rows.findIndex((r) => r.key === key);
  if (idx !== -1) rows.splice(idx, 1);
}

function scopeOptions(metric: ActivityMetric): { value: ClauseScope; label: string }[] {
  const info = catalog.metricInfo(metric);
  const options: { value: ClauseScope; label: string }[] = [];
  if (info?.supportsAggregateScope ?? true) {
    options.push({ value: ClauseScope.AGGREGATE, label: scopeLabels[ClauseScope.AGGREGATE] });
  }
  if (info?.supportsSingleScope) {
    options.push({ value: ClauseScope.SINGLE, label: scopeLabels[ClauseScope.SINGLE] });
  }
  return options;
}

function onMetricChange(row: ClauseRow, metric: ActivityMetric): void {
  row.metric = metric;
  const allowed = scopeOptions(metric).map((o) => o.value);
  if (!allowed.includes(row.scope) && allowed.length > 0) {
    row.scope = allowed[0];
  }
}

function rowValue(row: ClauseRow): number | null {
  const unit = catalog.metricInfo(row.metric)?.unit;
  if (unit === undefined) return null;
  return editStringToBaseValue(unit, row.valueText);
}

function rowError(row: ClauseRow): string {
  if (row.valueText.trim() === "") return "Value is required.";
  if (rowValue(row) === null) return "Enter a valid value.";
  return "";
}

// zod owns the structural "at least one clause" rule; per-row unit-aware
// value parsing (rowError above) isn't a good fit for zod since it depends
// on each row's metric unit, so it stays hand-written.
const clauseInputSchema = z.object({
  scope: z.number(),
  metric: z.number(),
  comparator: z.number(),
  value: z.number(),
});
const clauseListSchema = z.array(clauseInputSchema).min(1, "Add at least one clause.");

function currentClauses(): ClauseInput[] {
  return rows.map((r) => ({
    scope: r.scope,
    metric: r.metric,
    comparator: r.comparator,
    value: rowValue(r) ?? 0,
  }));
}

const cardinalityError = computed(() => {
  const result = clauseListSchema.safeParse(currentClauses());
  return result.success ? "" : result.error.issues[0]?.message ?? "Add at least one clause.";
});

const allValid = computed(
  () => cardinalityError.value === "" && rows.every((r) => rowError(r) === ""),
);

function emitChanges(): void {
  emit("update:modelValue", currentClauses());
  emit("update:valid", allValid.value);
}

watch(rows, emitChanges, { deep: true });
</script>

<template>
  <div class="space-y-3">
    <div v-if="!catalog.loaded" class="text-sm text-muted-foreground">Loading metrics…</div>
    <template v-else>
      <div
        v-for="row in rows"
        :key="row.key"
        class="rounded-lg border border-border p-3 space-y-3"
      >
        <div class="flex items-start justify-between gap-2">
          <div class="grid flex-1 grid-cols-1 gap-3 sm:grid-cols-2">
            <div class="space-y-1.5">
              <Label>Metric</Label>
              <Select
                :model-value="String(row.metric)"
                @update:model-value="(v) => onMetricChange(row, Number(v) as ActivityMetric)"
              >
                <SelectTrigger class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="m in catalog.metrics"
                    :key="m.key"
                    :value="String(m.metric)"
                  >
                    {{ m.label }} ({{ editUnitHint(m.unit) || "unitless" }})
                    <span v-if="m.nullable" class="text-muted-foreground"> · device-dependent</span>
                  </SelectItem>
                </SelectContent>
              </Select>
              <p
                v-if="catalog.metricInfo(row.metric)?.nullable"
                class="text-xs text-muted-foreground"
              >
                Not every activity reports this metric — activities missing it are
                excluded from the count.
              </p>
            </div>

            <div class="space-y-1.5">
              <Label>Scope</Label>
              <Select
                :model-value="String(row.scope)"
                @update:model-value="(v) => (row.scope = Number(v) as ClauseScope)"
              >
                <SelectTrigger class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="opt in scopeOptions(row.metric)"
                    :key="opt.value"
                    :value="String(opt.value)"
                  >
                    {{ opt.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div class="space-y-1.5">
              <Label>Comparator</Label>
              <Select
                :model-value="String(row.comparator)"
                @update:model-value="(v) => (row.comparator = Number(v) as Comparator)"
              >
                <SelectTrigger class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem :value="String(Comparator.GTE)">{{ comparatorLabels[Comparator.GTE] }}</SelectItem>
                  <SelectItem :value="String(Comparator.LTE)">{{ comparatorLabels[Comparator.LTE] }}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div class="space-y-1.5">
              <Label>Value ({{ editUnitHint(catalog.metricInfo(row.metric)?.unit ?? 0) || "number" }})</Label>
              <Input
                v-model="row.valueText"
                :placeholder="editUnitHint(catalog.metricInfo(row.metric)?.unit ?? 0)"
              />
              <p v-if="rowError(row)" class="text-xs text-destructive">{{ rowError(row) }}</p>
            </div>
          </div>

          <Button
            type="button"
            variant="outline"
            size="icon"
            :disabled="rows.length <= 1"
            title="Remove clause"
            @click="removeRow(row.key)"
          >
            <X class="size-4" />
          </Button>
        </div>
      </div>

      <Button type="button" variant="outline" size="sm" @click="addRow">
        <Plus class="size-4" /> Add clause
      </Button>
      <p v-if="cardinalityError" class="text-xs text-destructive">{{ cardinalityError }}</p>
    </template>
  </div>
</template>
