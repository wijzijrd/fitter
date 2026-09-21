<script setup lang="ts">
// SportSelectorField edits a fitter.v1.SportSelector: "all sports", a
// curated sport group from CatalogService.ListSports, or an exact Strava
// sport_type string.
import { computed, onMounted, ref, watch } from "vue";
import { useCatalogStore } from "@/stores/catalog";
import type { SportSelectorValue } from "@/lib/sportSelector";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const props = defineProps<{ modelValue: SportSelectorValue }>();
const emit = defineEmits<{ (e: "update:modelValue", value: SportSelectorValue): void }>();

const catalog = useCatalogStore();
onMounted(() => catalog.ensureLoaded());

const CUSTOM = "__custom__";
const ALL = "__all__";

const selectValue = computed(() => {
  if (props.modelValue.case === "group") return `group:${props.modelValue.value}`;
  if (props.modelValue.case === "sportType") return CUSTOM;
  return ALL;
});

const customText = ref(props.modelValue.case === "sportType" ? props.modelValue.value : "");

function onSelect(v: string): void {
  if (v === ALL) {
    emit("update:modelValue", { case: undefined });
  } else if (v === CUSTOM) {
    emit("update:modelValue", { case: "sportType", value: customText.value });
  } else if (v.startsWith("group:")) {
    emit("update:modelValue", { case: "group", value: v.slice("group:".length) });
  }
}

watch(customText, (v) => {
  if (selectValue.value === CUSTOM) {
    emit("update:modelValue", { case: "sportType", value: v });
  }
});
</script>

<template>
  <div class="space-y-1.5">
    <Label>Sport</Label>
    <Select :model-value="selectValue" @update:model-value="(v) => onSelect(String(v))">
      <SelectTrigger class="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem :value="ALL">All sports</SelectItem>
        <SelectItem v-for="g in catalog.sportGroups" :key="g.group" :value="`group:${g.group}`">
          {{ g.label }}
        </SelectItem>
        <SelectItem :value="CUSTOM">Specific sport type…</SelectItem>
      </SelectContent>
    </Select>
    <Input
      v-if="selectValue === CUSTOM"
      v-model="customText"
      placeholder="e.g. VirtualRide"
    />
  </div>
</template>
