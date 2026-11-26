<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { adminClient, exerciseClient } from "../lib/connect";
import { errorMessage } from "../lib/errors";
import { toDate } from "../lib/timestamp";
import type { AdminAction, AdminUser, CustomMetric, CustomSport, RegistrationAllowance } from "../gen/fitter/v1/admin_pb";
import { ExerciseMeasure, type Exercise, type Muscle } from "../gen/fitter/v1/exercise_pb";
import { measureLabels, measureOrder, muscleLabels, muscleOrder } from "../lib/labels";
import { useAuthStore } from "../stores/auth";
import { focusNthOr } from "../lib/focus";

const auth = useAuthStore();
type Tab = "users" | "catalog" | "exercises" | "audit";
const tabs: { id: Tab; label: string }[] = [
  { id: "users", label: "Users" },
  { id: "catalog", label: "Catalog" },
  { id: "exercises", label: "Exercises" },
  { id: "audit", label: "Audit" },
];
const tab = ref<Tab>("users");

// Same tab pattern as campaign detail: one tab stop, arrows and Home/End move between tabs.
function selectTab(ev: KeyboardEvent): void {
  const i = tabs.findIndex((t) => t.id === tab.value);
  const next = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: tabs.length - 1 }[ev.key];
  if (next === undefined) return;
  ev.preventDefault();
  tab.value = tabs[(next + tabs.length) % tabs.length].id;
  document.getElementById(`admin-tab-${tab.value}`)?.focus();
}
const error = ref("");
const busy = ref(false);

const users = ref<AdminUser[]>([]);
const sports = ref<CustomSport[]>([]);
const metrics = ref<CustomMetric[]>([]);
const actions = ref<AdminAction[]>([]);
const exercises = ref<Exercise[]>([]);

const pending = computed(() => users.value.filter((u) => u.status === "pending"));
const active = computed(() => users.value.filter((u) => u.status === "active"));
const banned = computed(() => users.value.filter((u) => u.status === "banned"));

async function run(fn: () => Promise<unknown>, reload: () => Promise<void>): Promise<void> {
  busy.value = true;
  error.value = "";
  try {
    await fn();
    await reload();
  } catch (err) {
    error.value = errorMessage(err, "That didn't work. Try again.");
  } finally {
    busy.value = false;
  }
}

const regMode = ref("open");
const regAllowed = ref<RegistrationAllowance[]>([]);
async function loadUsers(): Promise<void> {
  const [list, reg] = await Promise.all([adminClient.listUsers({}), adminClient.getRegistration({})]);
  users.value = list.users;
  regMode.value = reg.mode;
  regAllowed.value = reg.allowed;
}
async function loadCatalog(): Promise<void> {
  const [s, m] = await Promise.all([adminClient.listCustomSports({}), adminClient.listCustomMetrics({})]);
  sports.value = s.sports;
  metrics.value = m.metrics;
}
async function loadExercises(): Promise<void> {
  exercises.value = (await exerciseClient.listExercises({})).exercises;
}
async function loadAudit(): Promise<void> {
  actions.value = (await adminClient.listAdminActions({ limit: 200 })).actions;
}
const loaders: Record<Tab, () => Promise<void>> = { users: loadUsers, catalog: loadCatalog, exercises: loadExercises, audit: loadAudit };

async function load(): Promise<void> {
  error.value = "";
  try {
    await loaders[tab.value]();
  } catch (err) {
    error.value = errorMessage(err, "Couldn't load this tab. Try again.");
  }
}
onMounted(load);
watch(tab, load);

const act = (fn: () => Promise<unknown>) => run(fn, loaders[tab.value]);

const approve = (u: AdminUser) => act(() => adminClient.approveUser({ id: u.id }));
const unban = (u: AdminUser) => act(() => adminClient.unbanUser({ id: u.id }));
const setRole = (u: AdminUser, role: string) => act(() => adminClient.setUserRole({ id: u.id, role }));

const banning = ref("");
const banReason = ref("");
function startBan(u: AdminUser): void {
  banning.value = u.id;
  banReason.value = "";
}
async function confirmBan(u: AdminUser): Promise<void> {
  await act(() => adminClient.banUser({ id: u.id, reason: banReason.value.trim() }));
  if (error.value) return;
  banning.value = "";
  // The row moves to Banned; follow it to its Unban button.
  focusNthOr(`[data-unban="${u.id}"]`, 0, "#admin-banned");
}
function cancelBan(u: AdminUser): void {
  banning.value = "";
  focusNthOr(`[data-ban="${u.id}"]`, 0, "#admin-panel");
}

const setMode = (mode: string) => act(() => adminClient.setRegistrationMode({ mode }));
const allowId = ref("");
const allowIdValid = computed(() => /^\d{1,18}$/.test(allowId.value.trim()) && BigInt(allowId.value.trim()) > BigInt(0));
async function allowAthlete(): Promise<void> {
  await act(() => adminClient.allowAthlete({ stravaAthleteId: BigInt(allowId.value.trim()) }));
  if (!error.value) allowId.value = "";
}
const revokeAthlete = (a: RegistrationAllowance) => act(() => adminClient.revokeAthlete({ stravaAthleteId: a.stravaAthleteId }));

const sportForm = reactive({ key: "", label: "" });
const metricForm = reactive({
  key: "", label: "", unit: "", aggregation: "sum", nullable: false, lowerIsBetter: false,
  supportsSingleScope: true, supportsAggregateScope: true, refs: "",
});
async function createSport(): Promise<void> {
  await act(() => adminClient.createCustomSport({ key: sportForm.key.trim(), label: sportForm.label.trim() }));
  if (!error.value) Object.assign(sportForm, { key: "", label: "" });
}
async function createMetric(): Promise<void> {
  const { refs, ...rest } = metricForm;
  await act(() =>
    adminClient.createCustomMetric({
      ...rest,
      key: rest.key.trim(),
      label: rest.label.trim(),
      applicableSportRefs: refs.split(",").map((r) => r.trim()).filter(Boolean),
    }),
  );
  if (!error.value) Object.assign(metricForm, { key: "", label: "", unit: "", refs: "" });
}
// Two-step delete (DESIGN.md): the quiet Delete swaps for Keep / Delete; ids are "sport:<id>" / "metric:<id>".
const confirmDelete = ref("");
const kept = ref("");
async function deleteSport(s: CustomSport, i: number): Promise<void> {
  await act(() => adminClient.deleteCustomSport({ id: s.id }));
  if (!error.value) {
    confirmDelete.value = "";
    focusNthOr("[data-delete-sport]", i, "#admin-sports");
  }
}
async function deleteMetric(m: CustomMetric, i: number): Promise<void> {
  await act(() => adminClient.deleteCustomMetric({ id: m.id }));
  if (!error.value) {
    confirmDelete.value = "";
    focusNthOr("[data-delete-metric]", i, "#admin-metrics");
  }
}

// Exercises: edit in place, delete only when unused (two-step like the catalog), or merge a
// duplicate into the one to keep. One row is in one mode at a time.
const editing = ref("");
const merging = ref("");
const mergeTarget = ref("");
const mergeAsk = ref(false);
const editForm = reactive({ name: "", measure: ExerciseMeasure.WEIGHT_REPS, loaded: false, muscles: [] as Muscle[] });
const toggleEditMuscle = (m: Muscle) => {
  editForm.muscles = editForm.muscles.includes(m) ? editForm.muscles.filter((x) => x !== m) : [...editForm.muscles, m];
};
const optionalWeight = computed(() => editForm.measure === ExerciseMeasure.DISTANCE || editForm.measure === ExerciseMeasure.TIME);
const exerciseGroups = computed(() =>
  [
    { key: "active", title: "Exercise library", rows: exercises.value.filter((e) => !e.archived) },
    { key: "archived", title: "Archived", rows: exercises.value.filter((e) => e.archived) },
  ].filter((g) => g.key === "active" || g.rows.length),
);
const exerciseMeta = (e: Exercise) =>
  [measureLabels[e.measure] + (e.loaded ? " (weight optional)" : ""), e.muscles.map((m) => muscleLabels[m]).join(", ") || "no muscles yet", e.useCount ? `used ${e.useCount}×` : "unused"].join(" · ");
// A merge keeps the target's way of measuring: same measure, and it must take a weight if the source did.
const mergeTargets = (e: Exercise) => exercises.value.filter((x) => !x.archived && x.id !== e.id && x.measure === e.measure && (!e.loaded || x.loaded));

function startEdit(e: Exercise): void {
  merging.value = "";
  editing.value = e.id;
  Object.assign(editForm, { name: e.name, measure: e.measure, loaded: e.loaded, muscles: [...e.muscles] });
}
function cancelEdit(e: Exercise): void {
  editing.value = "";
  focusNthOr(`[data-edit="${e.id}"]`, 0, "#admin-exercises");
}
async function saveEdit(e: Exercise): Promise<void> {
  if (!editForm.name.trim() || !editForm.muscles.length) {
    error.value = !editForm.name.trim() ? "Name the exercise." : "Pick at least one muscle.";
    return;
  }
  await act(() => adminClient.updateExercise({ id: e.id, name: editForm.name, measure: editForm.measure, loaded: optionalWeight.value && editForm.loaded, muscles: editForm.muscles }));
  if (!error.value) {
    editing.value = "";
    focusNthOr(`[data-edit="${e.id}"]`, 0, "#admin-exercises");
  }
}
function startMerge(e: Exercise): void {
  editing.value = "";
  merging.value = e.id;
  mergeTarget.value = "";
  mergeAsk.value = false;
}
function cancelMerge(e: Exercise): void {
  merging.value = "";
  focusNthOr(`[data-merge="${e.id}"]`, 0, "#admin-exercises");
}
async function confirmMerge(e: Exercise): Promise<void> {
  await act(() => adminClient.mergeExercises({ sourceId: e.id, targetId: mergeTarget.value }));
  if (!error.value) {
    merging.value = "";
    focusNthOr("[data-edit]", 0, "#admin-exercises");
  }
}
// Archiving is reversible, so it doesn't ask: the row moves between lists and focus follows it.
async function setArchived(e: Exercise, archived: boolean): Promise<void> {
  await act(() => adminClient.setExerciseArchived({ id: e.id, archived }));
  if (!error.value) focusNthOr(`[data-${archived ? "restore" : "archive"}="${e.id}"]`, 0, "#admin-exercises");
}
async function deleteExercise(e: Exercise): Promise<void> {
  // Only unused rows have a Delete button, so focus follows the index among those.
  const i = exercises.value.filter((x) => !x.useCount).indexOf(e);
  await act(() => adminClient.deleteExercise({ id: e.id }));
  if (!error.value) {
    confirmDelete.value = "";
    focusNthOr("[data-delete-exercise]", i, "#admin-exercises");
  }
}

const fmtDate = (ts: Parameters<typeof toDate>[0]) => toDate(ts)?.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }) ?? "";
const isSelf = (u: AdminUser) => u.id === auth.user?.id;
</script>

<template>
  <main class="admin">
    <h1>Admin</h1>
    <div role="tablist" class="tabs" aria-label="Admin sections" @keydown="selectTab">
      <button
        v-for="t in tabs"
        :id="`admin-tab-${t.id}`"
        :key="t.id"
        type="button"
        role="tab"
        :aria-selected="tab === t.id"
        :tabindex="tab === t.id ? 0 : -1"
        aria-controls="admin-panel"
        @click="tab = t.id"
      >
        {{ t.label }}
      </button>
    </div>
    <p v-if="error" class="notice" role="alert">{{ error }}</p>

    <div id="admin-panel" role="tabpanel" tabindex="-1" :aria-labelledby="`admin-tab-${tab}`">
    <section v-if="tab === 'users'">
      <h2>Sign-ups</h2>
      <div class="rb-field">
        <label for="reg-mode">Who can join</label>
        <select id="reg-mode" class="input" :value="regMode" :disabled="busy" @change="setMode(($event.target as HTMLSelectElement).value)">
          <option value="open">Anyone who signs in with Strava, after approval</option>
          <option value="invite_only">Invite only: athletes listed below</option>
        </select>
      </div>
      <template v-if="regMode === 'invite_only'">
        <p v-if="!regAllowed.length" class="quiet">Nobody invited. Everyone new is refused at sign-in.</p>
        <ul class="rows">
          <li v-for="a in regAllowed" :key="a.stravaAthleteId.toString()">
            <span class="who"><span class="name">Strava athlete {{ a.stravaAthleteId }}</span><span class="meta">Invited {{ fmtDate(a.addedAt) }}</span></span>
            <button type="button" class="pill" v-busy="busy" @click="revokeAthlete(a)">Remove<span class="visually-hidden"> athlete {{ a.stravaAthleteId }}</span></button>
          </li>
        </ul>
        <form class="form" @submit.prevent="allowAthlete">
          <div class="rb-field">
            <label for="allow-id">Strava athlete id</label>
            <input id="allow-id" v-model="allowId" class="input" inputmode="numeric" autocomplete="off" placeholder="1234567" />
          </div>
          <button type="submit" class="pill pill--primary" v-busy="busy || !allowIdValid">Invite athlete</button>
        </form>
      </template>

      <h2>Pending ({{ pending.length }})</h2>
      <p v-if="!pending.length" class="quiet">Nobody waiting.</p>
      <ul class="rows">
        <li v-for="u in pending" :key="u.id">
          <span class="who"><span class="name">{{ u.name }}</span><span class="meta">Joined {{ fmtDate(u.createdAt) }}</span></span>
          <span class="actions">
            <button type="button" class="pill pill--primary" v-busy="busy" @click="approve(u)">Approve</button>
            <button type="button" class="pill" :data-ban="u.id" v-busy="busy" @click="startBan(u)">Ban…</button>
          </span>
          <form v-if="banning === u.id" class="ban" @submit.prevent="confirmBan(u)">
            <div class="rb-field">
              <label :for="`ban-${u.id}`">Reason for banning {{ u.name }}</label>
              <input :id="`ban-${u.id}`" v-focus="true" v-model="banReason" class="input" maxlength="500" required />
            </div>
            <span class="actions">
              <button type="button" class="pill" @click="cancelBan(u)">Cancel</button>
              <button type="submit" class="pill pill--primary" v-busy="busy || !banReason.trim()">Confirm ban</button>
            </span>
          </form>
        </li>
      </ul>

      <h2>Active ({{ active.length }})</h2>
      <ul class="rows">
        <li v-for="u in active" :key="u.id">
          <span class="who"><span class="name">{{ u.name }}</span><span class="meta">{{ u.role }}{{ isSelf(u) ? " · you" : "" }}</span></span>
          <span v-if="!isSelf(u)" class="actions">
            <button v-if="u.role === 'member'" type="button" class="pill" v-busy="busy" @click="setRole(u, 'admin')">Promote</button>
            <button v-else type="button" class="pill" v-busy="busy" @click="setRole(u, 'member')">Demote</button>
            <button v-if="u.role === 'member'" type="button" class="pill" :data-ban="u.id" v-busy="busy" @click="startBan(u)">Ban…</button>
          </span>
          <form v-if="banning === u.id" class="ban" @submit.prevent="confirmBan(u)">
            <div class="rb-field">
              <label :for="`ban-${u.id}`">Reason for banning {{ u.name }}</label>
              <input :id="`ban-${u.id}`" v-focus="true" v-model="banReason" class="input" maxlength="500" required />
            </div>
            <span class="actions">
              <button type="button" class="pill" @click="cancelBan(u)">Cancel</button>
              <button type="submit" class="pill pill--primary" v-busy="busy || !banReason.trim()">Confirm ban</button>
            </span>
          </form>
        </li>
      </ul>

      <h2 id="admin-banned" tabindex="-1">Banned ({{ banned.length }})</h2>
      <p v-if="!banned.length" class="quiet">No bans.</p>
      <ul class="rows">
        <li v-for="u in banned" :key="u.id">
          <span class="who"><span class="name">{{ u.name }}</span><span class="meta">{{ u.banReason }} · {{ fmtDate(u.bannedAt) }}</span></span>
          <button type="button" class="pill" :data-unban="u.id" v-busy="busy" @click="unban(u)">Unban</button>
        </li>
      </ul>
    </section>

    <section v-else-if="tab === 'catalog'">
      <h2 id="admin-sports" tabindex="-1">Custom sports</h2>
      <ul class="rows">
        <li v-for="(s, i) in sports" :key="s.id">
          <span class="who"><span class="name">{{ s.label }}</span><span class="meta">{{ s.key }}</span></span>
          <span v-if="confirmDelete === `sport:${s.id}`" class="confirm">
            <span>Delete this sport?</span>
            <span class="actions">
            <button v-focus="true" type="button" class="pill" @click="kept = `sport:${s.id}`; confirmDelete = ''">Keep</button>
            <button type="button" class="pill pill--primary" v-busy="busy" @click="deleteSport(s, i)">
              Delete<span class="visually-hidden"> {{ s.label }}</span>
            </button>
            </span>
          </span>
          <button v-else v-focus="kept === `sport:${s.id}`" type="button" class="pill pill--quiet" data-delete-sport @click="confirmDelete = `sport:${s.id}`">
            Delete<span class="visually-hidden"> {{ s.label }}</span>
          </button>
        </li>
      </ul>
      <form class="form" @submit.prevent="createSport">
        <div class="rb-field">
          <label for="sport-key">Key</label>
          <input id="sport-key" v-model="sportForm.key" class="input" required autocomplete="off" placeholder="powerlifting" />
        </div>
        <div class="rb-field">
          <label for="sport-label">Label</label>
          <input id="sport-label" v-model="sportForm.label" class="input" required autocomplete="off" placeholder="Powerlifting" />
        </div>
        <button type="submit" class="pill pill--primary" v-busy="busy">Add sport</button>
      </form>

      <h2 id="admin-metrics" tabindex="-1">Custom metrics</h2>
      <ul class="rows">
        <li v-for="(m, i) in metrics" :key="m.id">
          <span class="who"><span class="name">{{ m.label }}</span><span class="meta">{{ m.key }} · {{ m.aggregation }}{{ m.unit ? " · " + m.unit : "" }}</span></span>
          <span v-if="confirmDelete === `metric:${m.id}`" class="confirm">
            <span>Delete this metric?</span>
            <span class="actions">
            <button v-focus="true" type="button" class="pill" @click="kept = `metric:${m.id}`; confirmDelete = ''">Keep</button>
            <button type="button" class="pill pill--primary" v-busy="busy" @click="deleteMetric(m, i)">
              Delete<span class="visually-hidden"> {{ m.label }}</span>
            </button>
            </span>
          </span>
          <button v-else v-focus="kept === `metric:${m.id}`" type="button" class="pill pill--quiet" data-delete-metric @click="confirmDelete = `metric:${m.id}`">
            Delete<span class="visually-hidden"> {{ m.label }}</span>
          </button>
        </li>
      </ul>
      <form class="form" @submit.prevent="createMetric">
        <div class="rb-field">
          <label for="metric-key">Key</label>
          <input id="metric-key" v-model="metricForm.key" class="input" required autocomplete="off" />
        </div>
        <div class="rb-field">
          <label for="metric-label">Label</label>
          <input id="metric-label" v-model="metricForm.label" class="input" required autocomplete="off" />
        </div>
        <div class="rb-field">
          <label for="metric-unit">Unit <small>optional</small></label>
          <input id="metric-unit" v-model="metricForm.unit" class="input" autocomplete="off" placeholder="kg, reps" />
        </div>
        <div class="rb-field">
          <label for="metric-agg">Aggregation</label>
          <select id="metric-agg" v-model="metricForm.aggregation" class="input">
            <option>sum</option><option>average</option><option>max</option><option>count</option>
          </select>
        </div>
        <div class="rb-field form-wide">
          <label for="metric-refs">Applies to <small>optional</small></label>
          <input id="metric-refs" v-model="metricForm.refs" class="input" autocomplete="off" aria-describedby="metric-refs-hint" />
          <small id="metric-refs-hint">Sports or groups, comma separated. Leave blank for all.</small>
        </div>
        <fieldset class="checks form-wide">
          <legend class="visually-hidden">Metric options</legend>
          <label><input v-model="metricForm.nullable" type="checkbox" /> Nullable</label>
          <label><input v-model="metricForm.lowerIsBetter" type="checkbox" /> Lower is better</label>
          <label><input v-model="metricForm.supportsSingleScope" type="checkbox" /> Single scope</label>
          <label><input v-model="metricForm.supportsAggregateScope" type="checkbox" /> Aggregate scope</label>
        </fieldset>
        <button type="submit" class="pill pill--primary" v-busy="busy">Add metric</button>
      </form>
    </section>

    <section v-else-if="tab === 'exercises'">
      <template v-for="g in exerciseGroups" :key="g.key">
      <h2 :id="g.key === 'active' ? 'admin-exercises' : 'admin-exercises-archived'" tabindex="-1">{{ g.title }}</h2>
      <p v-if="g.key === 'active'" class="meta">Anyone can add an exercise from the log form. Merge moves a duplicate's history and goals onto the one to keep. Archive hides one that's in use from new logs; past sessions keep it.</p>
      <p v-else class="meta">Hidden from new logs and goal suggestions. Past sessions and goals still show them.</p>
      <ul class="rows">
        <li v-for="e in g.rows" :key="e.id">
          <span class="who"><span class="name">{{ e.name }}</span><span class="meta">{{ exerciseMeta(e) }}</span></span>

          <span v-if="confirmDelete === `exercise:${e.id}`" class="confirm">
            <span>Delete this exercise?</span>
            <span class="actions">
              <button v-focus="true" type="button" class="pill" @click="kept = `exercise:${e.id}`; confirmDelete = ''">Keep</button>
              <button type="button" class="pill pill--primary" v-busy="busy" @click="deleteExercise(e)">Delete<span class="visually-hidden"> {{ e.name }}</span></button>
            </span>
          </span>
          <span v-else class="actions">
            <button type="button" class="pill pill--quiet" :data-edit="e.id" @click="startEdit(e)">Edit<span class="visually-hidden"> {{ e.name }}</span></button>
            <button v-if="e.archived" type="button" class="pill pill--quiet" :data-restore="e.id" v-busy="busy" @click="setArchived(e, false)">Restore<span class="visually-hidden"> {{ e.name }}</span></button>
            <button v-if="e.useCount > 0" type="button" class="pill pill--quiet" :data-merge="e.id" @click="startMerge(e)">Merge<span class="visually-hidden"> {{ e.name }} into another</span></button>
            <button v-if="e.useCount > 0 && !e.archived" type="button" class="pill pill--quiet" :data-archive="e.id" v-busy="busy" @click="setArchived(e, true)">Archive<span class="visually-hidden"> {{ e.name }}</span></button>
            <button v-else-if="e.useCount === 0" v-focus="kept === `exercise:${e.id}`" type="button" class="pill pill--quiet" data-delete-exercise @click="confirmDelete = `exercise:${e.id}`">Delete<span class="visually-hidden"> {{ e.name }}</span></button>
          </span>

          <form v-if="editing === e.id" class="panel" @submit.prevent="saveEdit(e)" @keydown.esc="cancelEdit(e)">
            <div class="rb-field">
              <label :for="`ex-name-${e.id}`">Name</label>
              <input :id="`ex-name-${e.id}`" v-model="editForm.name" v-focus="true" class="input" maxlength="60" autocomplete="off" />
            </div>
            <div class="rb-field">
              <label :for="`ex-measure-${e.id}`">Measured by</label>
              <select :id="`ex-measure-${e.id}`" v-model="editForm.measure" class="input" :disabled="e.useCount > 0" :aria-describedby="e.useCount > 0 ? `ex-locked-${e.id}` : undefined">
                <option v-for="m in measureOrder" :key="m" :value="m">{{ measureLabels[m] }}</option>
              </select>
              <small v-if="e.useCount > 0" :id="`ex-locked-${e.id}`">Locked while it's in use. Merge into another exercise to change it.</small>
            </div>
            <label v-if="optionalWeight" class="choice choice--pill opt">
              <input v-model="editForm.loaded" type="checkbox" :disabled="e.useCount > 0" />
              <span>A weight is optional</span>
            </label>
            <fieldset class="rb-field">
              <legend>Muscles</legend>
              <div class="seg">
                <label v-for="m in muscleOrder" :key="m" class="choice choice--pill">
                  <input type="checkbox" :checked="editForm.muscles.includes(m)" @change="toggleEditMuscle(m)" />
                  <span>{{ muscleLabels[m] }}</span>
                </label>
              </div>
            </fieldset>
            <div class="actions">
              <button type="submit" class="pill pill--primary" v-busy="busy">Save</button>
              <button type="button" class="pill pill--quiet" @click="cancelEdit(e)">Cancel</button>
            </div>
          </form>

          <form v-if="merging === e.id" class="panel" @submit.prevent @keydown.esc="cancelMerge(e)">
            <div class="rb-field">
              <label :for="`ex-into-${e.id}`">Merge “{{ e.name }}” into</label>
              <select :id="`ex-into-${e.id}`" v-model="mergeTarget" v-focus="true" class="input" @change="mergeAsk = false">
                <option value="" disabled>Choose the one to keep…</option>
                <option v-for="t in mergeTargets(e)" :key="t.id" :value="t.id">{{ t.name }}</option>
              </select>
              <small v-if="!mergeTargets(e).length">Nothing else is measured the same way.</small>
            </div>
            <p v-if="mergeAsk" class="confirm-line">Move its {{ e.useCount }} uses, and any goals, to the one you chose? This can't be undone.</p>
            <div class="actions">
              <button v-if="!mergeAsk" type="button" class="pill" :aria-disabled="!mergeTarget" @click="mergeTarget && (mergeAsk = true)">Merge…</button>
              <button v-else type="button" class="pill pill--primary" v-busy="busy" @click="confirmMerge(e)">Merge<span class="visually-hidden"> {{ e.name }}</span></button>
              <button type="button" class="pill pill--quiet" @click="cancelMerge(e)">Cancel</button>
            </div>
          </form>
        </li>
      </ul>
      </template>
    </section>

    <section v-else>
      <h2>Audit log</h2>
      <p v-if="!actions.length" class="quiet">Nothing recorded yet.</p>
      <div v-else class="table-wrap" tabindex="0" role="region" aria-label="Audit log table">
      <table>
        <thead><tr><th>When</th><th>Admin</th><th>Action</th><th>Target</th><th>Detail</th></tr></thead>
        <tbody>
          <tr v-for="a in actions" :key="a.id">
            <td>{{ fmtDate(a.createdAt) }}</td><td>{{ a.actorName }}</td><td>{{ a.action }}</td>
            <td class="mono">{{ a.targetId }}</td><td>{{ a.detail }}</td>
          </tr>
        </tbody>
      </table>
      </div>
    </section>
    </div>
  </main>
</template>

<style scoped>
.admin {
  max-width: 960px;
  margin: 0 auto;
  padding: var(--space-5) var(--gutter) var(--space-8);
  font-family: var(--font-body);
  color: var(--ink);
}
h1 {
  margin: 0 0 var(--space-5);
  font: 400 clamp(1.625rem, 6.5vw, 2rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}
/* Group labels share the standings' label style (DESIGN.md). */
h2 {
  margin: var(--group-gap) 0 var(--space-3);
  font: 400 0.875rem / 1.3 var(--font-display);
  text-transform: uppercase;
}
/* Underline tabs, as on campaign detail (DESIGN.md Navigation). */
.tabs {
  display: flex;
  gap: var(--space-7);
  border-bottom: 2px solid var(--hairline);
  margin-bottom: var(--space-5);
}
.tabs button {
  min-height: 2.75rem;
  border: 0;
  background: none;
  padding: var(--space-3) 0 var(--row);
  margin-bottom: -2px;
  border-bottom: 3px solid transparent;
  color: var(--ink-muted);
  font: 700 1rem var(--font-body);
  cursor: pointer;
}
.tabs button[aria-selected="true"] {
  color: var(--ink);
  border-bottom-color: var(--tangerine);
}
.notice {
  padding: var(--space-4) var(--space-5);
  border: 2px solid var(--line);
  border-radius: 10px;
  background: var(--cool-blue);
  font-weight: 700;
}
.quiet,
.meta {
  color: var(--ink-muted);
}
.rows {
  list-style: none;
  margin: 0;
  padding: 0;
}
.rows li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3) var(--space-4);
  padding: var(--row) 0;
  border-bottom: 1px solid var(--hairline);
}
.name {
  font-weight: 700;
}
/* Name over meta, actions to the right; on a phone the actions wrap under the name. */
.who {
  display: flex;
  flex: 1 1 12rem;
  flex-direction: column;
  min-width: 0;
}
.who .name,
.who .meta {
  overflow-wrap: anywhere;
}
.ban {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: var(--space-3);
  width: 100%;
}
.ban .rb-field {
  flex: 1 1 100%;
  margin: 0;
}
.form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0 var(--space-5);
  align-items: end;
  margin-top: var(--space-5);
}
.form-wide {
  grid-column: 1 / -1;
}
.form > .pill {
  justify-self: start;
  margin-bottom: var(--space-6);
}
.checks {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3) var(--space-6);
  margin: 0 0 var(--space-6);
  padding: 0;
  border: 0;
}
.checks label {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 2.75rem;
  cursor: pointer;
}
.checks input {
  width: 1.05rem;
  height: 1.05rem;
  accent-color: var(--ink);
}
.panel {
  flex: 1 1 100%;
  padding: var(--row) var(--bib-inset);
  border: 2px solid var(--line);
  border-radius: 10px;
}
.panel fieldset {
  border: 0;
  padding: 0;
  min-width: 0;
}
.seg {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.opt {
  margin-bottom: var(--space-5);
}
.confirm-line {
  margin: 0 0 var(--space-4);
  font-weight: 700;
}
.confirm {
  display: flex;
  flex: 1 1 16rem;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-2);
  font-weight: 700;
}
.confirm > span:first-child {
  margin-right: auto;
}
.actions {
  display: flex;
  gap: var(--space-2);
}
.table-wrap {
  overflow-x: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}
th,
td {
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--hairline);
  text-align: left;
  vertical-align: top;
}
th {
  border-bottom: 2px solid var(--line);
}
.mono {
  font-family: ui-monospace, monospace;
  font-size: 0.75rem;
  overflow-wrap: anywhere;
}
</style>
