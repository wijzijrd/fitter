<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import { z } from "zod";
import { useCampaignsStore } from "../stores/campaigns";
import { useCatalogStore } from "../stores/catalog";
import { ActivityMetric } from "../gen/fitter/v1/common_pb";
import { editStringToBaseValue, editUnitHint, formatMetricValue } from "../lib/metricFormat";
import { toDate } from "../lib/timestamp";
import { allSports } from "../lib/sportSelector";
import type { GoalFormCore } from "../components/GoalForm.vue";
import GoalForm from "../components/GoalForm.vue";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Plus, X } from "@lucide/vue";

const store = useCampaignsStore();
const catalog = useCatalogStore();

const listError = ref("");

onMounted(async () => {
  await catalog.ensureLoaded();
  try {
    await store.fetchCampaigns();
  } catch (e) {
    listError.value = e instanceof Error ? e.message : "Failed to load campaigns.";
  }
});

// --- create campaign dialog ---------------------------------------------

const NO_TARGET = "none";

const createSchema = toTypedSchema(
  z
    .object({
      name: z.string().min(1, "Name is required"),
      description: z.string(),
      metric: z.string(),
      targetValue: z.string(),
      startDate: z.string().min(1, "Start date is required"),
      endDate: z.string().min(1, "End date is required"),
    })
    .refine((v) => v.metric === NO_TARGET || v.targetValue.trim() !== "", {
      message: "Enter a target value",
      path: ["targetValue"],
    })
    .refine((v) => new Date(v.endDate) > new Date(v.startDate), {
      message: "End date must be after start date",
      path: ["endDate"],
    }),
);

function createDefaults() {
  return {
    name: "",
    description: "",
    metric: NO_TARGET,
    targetValue: "",
    startDate: new Date().toISOString().slice(0, 10),
    endDate: new Date(Date.now() + 30 * 86400_000).toISOString().slice(0, 10),
  };
}

const { defineField, errors, handleSubmit, resetForm, values } = useForm({
  validationSchema: createSchema,
  initialValues: createDefaults(),
});
const [name] = defineField("name");
const [description] = defineField("description");
const [metric] = defineField("metric");
const [targetValue] = defineField("targetValue");
const [startDate] = defineField("startDate");
const [endDate] = defineField("endDate");

const showCreate = ref(false);
const createError = ref("");
const creating = ref(false);

const targetUnitHint = computed(() => {
  if (values.metric === NO_TARGET) return "";
  return editUnitHint(catalog.metricInfo(Number(values.metric) as ActivityMetric)?.unit ?? 0);
});

const submitCreate = handleSubmit(async (v) => {
  createError.value = "";
  creating.value = true;
  try {
    const hasTarget = v.metric !== NO_TARGET;
    const metricEnum = hasTarget ? (Number(v.metric) as ActivityMetric) : undefined;
    const targetBase = hasTarget
      ? editStringToBaseValue(catalog.metricInfo(metricEnum!)?.unit ?? 0, v.targetValue) ?? undefined
      : undefined;
    await store.createCampaign({
      name: v.name,
      description: v.description,
      metric: metricEnum,
      targetValue: targetBase,
      startDate: new Date(v.startDate),
      endDate: new Date(v.endDate),
    });
    showCreate.value = false;
    resetForm({ values: createDefaults() });
  } catch (e) {
    createError.value = e instanceof Error ? e.message : "Failed to create campaign.";
  } finally {
    creating.value = false;
  }
});

// --- join campaign dialog ------------------------------------------------

function blankGoal(): GoalFormCore {
  return { name: "", sport: allSports(), clauses: [] };
}

const showJoin = ref(false);
const joiningCampaignId = ref<string | null>(null);
const joinGoals = ref<GoalFormCore[]>([blankGoal()]);
const joinValidFlags = ref<boolean[]>([false]);
const joinError = ref("");
const joining = ref(false);

function openJoin(campaignId: string): void {
  joiningCampaignId.value = campaignId;
  joinGoals.value = [blankGoal()];
  joinValidFlags.value = [false];
  joinError.value = "";
  showJoin.value = true;
}

function addJoinGoal(): void {
  joinGoals.value.push(blankGoal());
  joinValidFlags.value.push(false);
}

function removeJoinGoal(i: number): void {
  joinGoals.value.splice(i, 1);
  joinValidFlags.value.splice(i, 1);
}

const joinFormValid = computed(
  () => joinGoals.value.length > 0 && joinValidFlags.value.every(Boolean),
);

async function submitJoin(): Promise<void> {
  if (!joiningCampaignId.value || !joinFormValid.value) return;
  joining.value = true;
  joinError.value = "";
  try {
    await store.joinCampaign(joiningCampaignId.value, joinGoals.value);
    showJoin.value = false;
  } catch (e) {
    joinError.value = e instanceof Error ? e.message : "Failed to join campaign.";
  } finally {
    joining.value = false;
  }
}

const leaveError = ref("");

async function leave(campaignId: string): Promise<void> {
  leaveError.value = "";
  try {
    await store.leaveCampaign(campaignId);
  } catch (e) {
    leaveError.value = e instanceof Error ? e.message : "Failed to leave campaign.";
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-4 p-6">
    <div class="flex items-center justify-between">
      <h2 class="text-xl font-semibold">Campaigns</h2>
      <Dialog v-model:open="showCreate">
        <DialogTrigger as-child>
          <Button>New campaign</Button>
        </DialogTrigger>
        <DialogContent class="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>New campaign</DialogTitle>
          </DialogHeader>
          <form class="space-y-4" @submit="submitCreate">
            <div class="space-y-1.5">
              <Label>Name</Label>
              <Input v-model="name" required />
              <p v-if="errors.name" class="text-sm text-destructive">{{ errors.name }}</p>
            </div>
            <div class="space-y-1.5">
              <Label>Description</Label>
              <Textarea v-model="description" rows="2" />
            </div>
            <div class="space-y-1.5">
              <Label>Pooled target metric (optional)</Label>
              <Select v-model="metric">
                <SelectTrigger class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem :value="NO_TARGET">No pooled target — personal goals only</SelectItem>
                  <SelectItem v-for="m in catalog.metrics" :key="m.key" :value="String(m.metric)">
                    {{ m.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div v-if="metric !== NO_TARGET" class="space-y-1.5">
              <Label>Target value ({{ targetUnitHint }})</Label>
              <Input v-model="targetValue" :placeholder="targetUnitHint" />
              <p v-if="errors.targetValue" class="text-sm text-destructive">{{ errors.targetValue }}</p>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div class="space-y-1.5">
                <Label>Start date</Label>
                <Input v-model="startDate" type="date" required />
              </div>
              <div class="space-y-1.5">
                <Label>End date</Label>
                <Input v-model="endDate" type="date" required />
                <p v-if="errors.endDate" class="text-sm text-destructive">{{ errors.endDate }}</p>
              </div>
            </div>
            <Alert v-if="createError" variant="destructive">
              <AlertDescription>{{ createError }}</AlertDescription>
            </Alert>
            <DialogFooter>
              <Button type="submit" :disabled="creating">{{ creating ? "Creating…" : "Create" }}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>

    <Alert v-if="listError" variant="destructive">
      <AlertDescription>{{ listError }}</AlertDescription>
    </Alert>
    <Alert v-if="leaveError" variant="destructive">
      <AlertDescription>{{ leaveError }}</AlertDescription>
    </Alert>

    <p v-if="store.loading" class="text-sm text-muted-foreground">Loading…</p>

    <Card v-for="c in store.campaigns" :key="c.id">
      <CardHeader class="flex flex-row items-start justify-between gap-3">
        <div>
          <router-link :to="`/campaigns/${c.id}`" class="font-semibold hover:underline">
            <CardTitle>{{ c.name }}</CardTitle>
          </router-link>
          <p class="mt-1 text-sm text-muted-foreground">
            {{ toDate(c.startDate)?.toLocaleDateString() }} – {{ toDate(c.endDate)?.toLocaleDateString() }}
            · {{ c.memberCount }} members · {{ c.goalCount }} goals
          </p>
          <Badge v-if="c.metric !== undefined" variant="secondary" class="mt-2">
            Pooled target: {{ catalog.metricLabel(c.metric) }}
            {{ formatMetricValue(catalog.metricInfo(c.metric)?.unit ?? 0, c.targetValue ?? 0) }}
          </Badge>
        </div>
        <Button v-if="!c.joined" size="sm" @click="openJoin(c.id)">Join</Button>
        <Button v-else variant="outline" size="sm" @click="leave(c.id)">Leave</Button>
      </CardHeader>
      <CardContent v-if="c.description">
        <p class="text-sm text-muted-foreground">{{ c.description }}</p>
      </CardContent>
    </Card>
    <p v-if="!store.loading && store.campaigns.length === 0" class="text-sm text-muted-foreground">
      No campaigns yet — create one.
    </p>

    <Dialog v-model:open="showJoin">
      <DialogContent class="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Join campaign</DialogTitle>
        </DialogHeader>
        <div class="max-h-[60vh] space-y-4 overflow-y-auto">
          <div
            v-for="(goal, i) in joinGoals"
            :key="i"
            class="space-y-2 rounded-lg border border-border p-3"
          >
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">Goal {{ i + 1 }}</span>
              <Button
                v-if="joinGoals.length > 1"
                type="button"
                variant="outline"
                size="icon"
                @click="removeJoinGoal(i)"
              >
                <X class="size-4" />
              </Button>
            </div>
            <GoalForm
              :model-value="goal"
              @update:model-value="(v) => (joinGoals[i] = v)"
              @update:valid="(v) => (joinValidFlags[i] = v)"
            />
          </div>
          <Button type="button" variant="outline" size="sm" @click="addJoinGoal">
            <Plus class="size-4" /> Add another goal
          </Button>
          <Alert v-if="joinError" variant="destructive">
            <AlertDescription>{{ joinError }}</AlertDescription>
          </Alert>
        </div>
        <DialogFooter>
          <Button :disabled="!joinFormValid || joining" @click="submitJoin">
            {{ joining ? "Joining…" : "Join campaign" }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
