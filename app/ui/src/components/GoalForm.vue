<script setup lang="ts">
// GoalForm is the "name + sport + clauses" core of a goal, reused by both
// GoalsView's personal-goal create dialog and CampaignsView's join-campaign
// flow (one instance per goal being created there). Period dates are
// handled by the caller when relevant (personal goals only — a
// join-campaign goal's period is always forced to the campaign window
// server-side, see lib/goalInput.ts).
import { reactive, watch } from "vue";
import type { ClauseInput } from "@/lib/clauses";
import type { SportSelectorValue } from "@/lib/sportSelector";
import { allSports } from "@/lib/sportSelector";
import ClauseBuilder from "@/components/ClauseBuilder.vue";
import SportSelectorField from "@/components/SportSelectorField.vue";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";

export interface GoalFormCore {
  name: string;
  sport: SportSelectorValue;
  clauses: ClauseInput[];
}

const props = defineProps<{ modelValue: GoalFormCore }>();
const emit = defineEmits<{
  (e: "update:modelValue", value: GoalFormCore): void;
  (e: "update:valid", valid: boolean): void;
}>();

const local = reactive<GoalFormCore>({
  name: props.modelValue.name,
  sport: props.modelValue.sport ?? allSports(),
  clauses: props.modelValue.clauses,
});

let clausesValid = local.clauses.length > 0;

function emitChange(): void {
  emit("update:modelValue", { name: local.name, sport: local.sport, clauses: local.clauses });
  emit("update:valid", local.name.trim() !== "" && clausesValid);
}

function onClausesUpdate(clauses: ClauseInput[]): void {
  local.clauses = clauses;
  emitChange();
}

function onClausesValid(valid: boolean): void {
  clausesValid = valid;
  emitChange();
}

watch(() => local.name, emitChange);
watch(() => local.sport, emitChange, { deep: true });
</script>

<template>
  <div class="space-y-3">
    <div class="space-y-1.5">
      <Label>Name</Label>
      <Input v-model="local.name" placeholder="e.g. 30-day run volume" required />
    </div>
    <SportSelectorField v-model="local.sport" />
    <div class="space-y-1.5">
      <Label>Clauses</Label>
      <ClauseBuilder
        :model-value="local.clauses"
        @update:model-value="onClausesUpdate"
        @update:valid="onClausesValid"
      />
    </div>
  </div>
</template>
