<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import { z } from "zod";
import { useGoalsStore } from "../stores/goals";
import { useCatalogStore } from "../stores/catalog";
import { sportSelectorLabel } from "../lib/labels";
import { allSports } from "../lib/sportSelector";
import type { SportSelectorValue } from "../lib/sportSelector";
import { toDate } from "../lib/timestamp";
import type { Goal } from "../gen/fitter/v1/goal_pb";
import { statusTier, badgeVariantForTier } from "../lib/statusTier";
import type { GoalFormCore } from "../components/GoalForm.vue";
import GoalForm from "../components/GoalForm.vue";
import ClauseProgressList from "../components/ClauseProgressList.vue";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";

const store = useGoalsStore();
const catalog = useCatalogStore();

const listError = ref("");

onMounted(async () => {
  await catalog.ensureLoaded();
  try {
    await store.fetchGoals();
  } catch (e) {
    listError.value = e instanceof Error ? e.message : "Failed to load goals.";
  }
});

// --- create goal dialog ---------------------------------------------------

const periodSchema = toTypedSchema(
  z
    .object({
      periodStart: z.string().min(1, "Start date is required"),
      periodEnd: z.string().min(1, "End date is required"),
    })
    .refine((v) => new Date(v.periodEnd) > new Date(v.periodStart), {
      message: "End date must be after start date",
      path: ["periodEnd"],
    }),
);

function periodDefaults() {
  return {
    periodStart: new Date().toISOString().slice(0, 10),
    periodEnd: new Date(Date.now() + 30 * 86400_000).toISOString().slice(0, 10),
  };
}

const { defineField, errors, handleSubmit, resetForm } = useForm({
  validationSchema: periodSchema,
  initialValues: periodDefaults(),
});
const [periodStart] = defineField("periodStart");
const [periodEnd] = defineField("periodEnd");

function blankGoalCore(): GoalFormCore {
  return { name: "", sport: allSports(), clauses: [] };
}

const showCreate = ref(false);
const creating = ref(false);
const createError = ref("");
const goalCore = ref<GoalFormCore>(blankGoalCore());
const goalCoreValid = ref(false);
const formKey = ref(0);

function openCreate(): void {
  goalCore.value = blankGoalCore();
  goalCoreValid.value = false;
  createError.value = "";
  resetForm({ values: periodDefaults() });
  formKey.value += 1; // remounts GoalForm/ClauseBuilder so their internal state resets too
  showCreate.value = true;
}

const submitCreate = handleSubmit(async (v) => {
  if (!goalCoreValid.value) {
    createError.value = "Add a name and at least one valid clause.";
    return;
  }
  creating.value = true;
  createError.value = "";
  try {
    await store.createGoal({
      name: goalCore.value.name,
      sport: goalCore.value.sport,
      clauses: goalCore.value.clauses,
      periodStart: new Date(v.periodStart),
      periodEnd: new Date(v.periodEnd),
    });
    showCreate.value = false;
  } catch (e) {
    createError.value = e instanceof Error ? e.message : "Failed to create goal.";
  } finally {
    creating.value = false;
  }
});

// --- edit goal dialog ------------------------------------------------------

const {
  defineField: defineEditField,
  errors: editErrors,
  handleSubmit: handleEditSubmit,
  resetForm: resetEditForm,
} = useForm({
  validationSchema: periodSchema,
  initialValues: periodDefaults(),
});
const [editPeriodStart] = defineEditField("periodStart");
const [editPeriodEnd] = defineEditField("periodEnd");

function goalToCore(g: Goal): GoalFormCore {
  const sport: SportSelectorValue = g.sport?.selector ?? allSports();
  return {
    name: g.name,
    sport,
    clauses: g.clauses.map((c) => ({
      scope: c.scope,
      metric: c.metric,
      comparator: c.comparator,
      value: c.value,
    })),
  };
}

function dateInputValue(d: Date | null | undefined): string {
  return d ? d.toISOString().slice(0, 10) : periodDefaults().periodStart;
}

const showEdit = ref(false);
const editing = ref(false);
const editError = ref("");
const editGoalId = ref<string | null>(null);
const editGoalCore = ref<GoalFormCore>(blankGoalCore());
const editGoalCoreValid = ref(false);
const editFormKey = ref(0);

function openEdit(g: Goal): void {
  editGoalId.value = g.id;
  editGoalCore.value = goalToCore(g);
  editGoalCoreValid.value = false;
  editError.value = "";
  resetEditForm({
    values: {
      periodStart: dateInputValue(toDate(g.periodStart)),
      periodEnd: dateInputValue(toDate(g.periodEnd)),
    },
  });
  editFormKey.value += 1; // remounts GoalForm/ClauseBuilder so they seed from this goal, not a previous one
  showEdit.value = true;
}

const submitEdit = handleEditSubmit(async (v) => {
  if (!editGoalId.value) return;
  if (!editGoalCoreValid.value) {
    editError.value = "Add a name and at least one valid clause.";
    return;
  }
  editing.value = true;
  editError.value = "";
  try {
    await store.updateGoal(editGoalId.value, {
      name: editGoalCore.value.name,
      sport: editGoalCore.value.sport,
      clauses: editGoalCore.value.clauses,
      periodStart: new Date(v.periodStart),
      periodEnd: new Date(v.periodEnd),
    });
    showEdit.value = false;
  } catch (e) {
    editError.value = e instanceof Error ? e.message : "Failed to update goal.";
  } finally {
    editing.value = false;
  }
});

// --- delete with confirmation ---------------------------------------------

const deleteTargetId = ref<string | null>(null);
const deleteError = ref("");
const deleting = ref(false);

function confirmDelete(goalId: string): void {
  deleteTargetId.value = goalId;
  deleteError.value = "";
}

async function doDelete(): Promise<void> {
  if (!deleteTargetId.value) return;
  deleting.value = true;
  try {
    await store.deleteGoal(deleteTargetId.value);
    deleteTargetId.value = null;
  } catch (e) {
    deleteError.value = e instanceof Error ? e.message : "Failed to delete goal.";
  } finally {
    deleting.value = false;
  }
}

const deleteDialogOpen = computed({
  get: () => deleteTargetId.value !== null,
  set: (v: boolean) => {
    if (!v) deleteTargetId.value = null;
  },
});
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-4 p-6">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-xl font-semibold">My goals</h2>
        <p class="text-sm text-muted-foreground">Personal goals are private — only you can see them.</p>
      </div>
      <Dialog v-model:open="showCreate">
        <DialogTrigger as-child>
          <Button @click="openCreate">New goal</Button>
        </DialogTrigger>
        <DialogContent class="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>New goal</DialogTitle>
          </DialogHeader>
          <form :key="formKey" class="space-y-4" @submit="submitCreate">
            <GoalForm
              v-model="goalCore"
              @update:valid="(v) => (goalCoreValid = v)"
            />
            <div class="grid grid-cols-2 gap-3">
              <div class="space-y-1.5">
                <Label>Period start</Label>
                <Input v-model="periodStart" type="date" required />
                <p v-if="errors.periodStart" class="text-sm text-destructive">{{ errors.periodStart }}</p>
              </div>
              <div class="space-y-1.5">
                <Label>Period end</Label>
                <Input v-model="periodEnd" type="date" required />
                <p v-if="errors.periodEnd" class="text-sm text-destructive">{{ errors.periodEnd }}</p>
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
    <Alert v-if="deleteError" variant="destructive">
      <AlertDescription>{{ deleteError }}</AlertDescription>
    </Alert>

    <Card v-for="g in store.goals" :key="g.id">
      <CardHeader class="flex flex-row items-start justify-between gap-3">
        <div>
          <CardTitle>{{ g.name }}</CardTitle>
          <p class="text-sm text-muted-foreground">
            {{ sportSelectorLabel(g.sport, catalog.sportGroups) }} ·
            {{ toDate(g.periodStart)?.toLocaleDateString() }} – {{ toDate(g.periodEnd)?.toLocaleDateString() }}
          </p>
          <Badge
            :variant="badgeVariantForTier(statusTier(g.progress?.satisfied ?? false, g.progress?.progressRatio ?? 0))"
            class="mt-2"
          >
            {{ g.progress?.satisfied ? "Goal met" : "In progress" }}
          </Badge>
        </div>
        <div class="flex shrink-0 gap-2">
          <Button variant="outline" size="sm" @click="openEdit(g)">Edit</Button>
          <Button variant="outline" size="sm" @click="confirmDelete(g.id)">Delete</Button>
        </div>
      </CardHeader>
      <CardContent>
        <ClauseProgressList :clauses="g.clauses" :progress="g.progress?.clauses" />
      </CardContent>
    </Card>
    <p v-if="store.goals.length === 0" class="text-sm text-muted-foreground">No goals yet.</p>

    <Dialog v-model:open="showEdit">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Edit goal</DialogTitle>
        </DialogHeader>
        <form :key="editFormKey" class="space-y-4" @submit="submitEdit">
          <GoalForm
            v-model="editGoalCore"
            @update:valid="(v) => (editGoalCoreValid = v)"
          />
          <div class="grid grid-cols-2 gap-3">
            <div class="space-y-1.5">
              <Label>Period start</Label>
              <Input v-model="editPeriodStart" type="date" required />
              <p v-if="editErrors.periodStart" class="text-sm text-destructive">{{ editErrors.periodStart }}</p>
            </div>
            <div class="space-y-1.5">
              <Label>Period end</Label>
              <Input v-model="editPeriodEnd" type="date" required />
              <p v-if="editErrors.periodEnd" class="text-sm text-destructive">{{ editErrors.periodEnd }}</p>
            </div>
          </div>
          <Alert v-if="editError" variant="destructive">
            <AlertDescription>{{ editError }}</AlertDescription>
          </Alert>
          <DialogFooter>
            <Button type="submit" :disabled="editing">{{ editing ? "Saving…" : "Save" }}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete this goal?</AlertDialogTitle>
          <AlertDialogDescription>
            This can't be undone. Progress and history for this goal will be lost.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction :disabled="deleting" @click="doDelete">
            {{ deleting ? "Deleting…" : "Delete" }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
