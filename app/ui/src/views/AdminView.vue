<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { adminClient } from "../lib/connect";
import { errorMessage } from "../lib/errors";
import { toDate } from "../lib/timestamp";
import type { AdminAction, AdminUser, CustomMetric, CustomSport } from "../gen/fitter/v1/admin_pb";
import { useAuthStore } from "../stores/auth";
import { focusNthOr } from "../lib/focus";

const auth = useAuthStore();
type Tab = "users" | "catalog" | "audit";
const tabs: { id: Tab; label: string }[] = [
  { id: "users", label: "Users" },
  { id: "catalog", label: "Catalog" },
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

async function loadUsers(): Promise<void> {
  users.value = (await adminClient.listUsers({})).users;
}
async function loadCatalog(): Promise<void> {
  const [s, m] = await Promise.all([adminClient.listCustomSports({}), adminClient.listCustomMetrics({})]);
  sports.value = s.sports;
  metrics.value = m.metrics;
}
async function loadAudit(): Promise<void> {
  actions.value = (await adminClient.listAdminActions({ limit: 200 })).actions;
}
const loaders: Record<Tab, () => Promise<void>> = { users: loadUsers, catalog: loadCatalog, audit: loadAudit };

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
  if (!error.value) banning.value = "";
}

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

const fmtDate = (ts: Parameters<typeof toDate>[0]) => toDate(ts)?.toLocaleString() ?? "";
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

    <div id="admin-panel" role="tabpanel" :aria-labelledby="`admin-tab-${tab}`">
    <section v-if="tab === 'users'">
      <h2>Pending ({{ pending.length }})</h2>
      <p v-if="!pending.length" class="quiet">Nobody waiting.</p>
      <ul class="rows">
        <li v-for="u in pending" :key="u.id">
          <span class="name">{{ u.name }}</span>
          <span class="meta">Joined {{ fmtDate(u.createdAt) }}</span>
          <button type="button" class="pill pill--primary" v-busy="busy" @click="approve(u)">Approve</button>
          <button type="button" class="pill" v-busy="busy" @click="startBan(u)">Ban…</button>
          <form v-if="banning === u.id" class="ban" @submit.prevent="confirmBan(u)">
            <div class="rb-field">
              <label :for="`ban-${u.id}`">Reason for banning {{ u.name }}</label>
              <input :id="`ban-${u.id}`" v-model="banReason" class="input" maxlength="500" required />
            </div>
            <button type="button" class="pill" @click="banning = ''">Cancel</button>
            <button type="submit" class="pill pill--primary" v-busy="busy || !banReason.trim()">Confirm ban</button>
          </form>
        </li>
      </ul>

      <h2>Active ({{ active.length }})</h2>
      <ul class="rows">
        <li v-for="u in active" :key="u.id">
          <span class="name">{{ u.name }}</span>
          <span class="meta">{{ u.role }}</span>
          <template v-if="!isSelf(u)">
            <button v-if="u.role === 'member'" type="button" class="pill" v-busy="busy" @click="setRole(u, 'admin')">Promote</button>
            <button v-else type="button" class="pill" v-busy="busy" @click="setRole(u, 'member')">Demote</button>
            <button v-if="u.role === 'member'" type="button" class="pill" v-busy="busy" @click="startBan(u)">Ban…</button>
          </template>
          <span v-else class="meta">you</span>
          <form v-if="banning === u.id" class="ban" @submit.prevent="confirmBan(u)">
            <div class="rb-field">
              <label :for="`ban-${u.id}`">Reason for banning {{ u.name }}</label>
              <input :id="`ban-${u.id}`" v-model="banReason" class="input" maxlength="500" required />
            </div>
            <button type="button" class="pill" @click="banning = ''">Cancel</button>
            <button type="submit" class="pill pill--primary" v-busy="busy || !banReason.trim()">Confirm ban</button>
          </form>
        </li>
      </ul>

      <h2>Banned ({{ banned.length }})</h2>
      <p v-if="!banned.length" class="quiet">No bans.</p>
      <ul class="rows">
        <li v-for="u in banned" :key="u.id">
          <span class="name">{{ u.name }}</span>
          <span class="meta">{{ u.banReason }} · {{ fmtDate(u.bannedAt) }}</span>
          <button type="button" class="pill" v-busy="busy" @click="unban(u)">Unban</button>
        </li>
      </ul>
    </section>

    <section v-else-if="tab === 'catalog'">
      <h2 id="admin-sports" tabindex="-1">Custom sports</h2>
      <ul class="rows">
        <li v-for="(s, i) in sports" :key="s.id">
          <span class="name">{{ s.label }}</span><span class="meta">{{ s.key }}</span>
          <span v-if="confirmDelete === `sport:${s.id}`" class="confirm">
            <span>Delete this sport?</span>
            <button v-focus="true" type="button" class="pill" @click="kept = `sport:${s.id}`; confirmDelete = ''">Keep</button>
            <button type="button" class="pill pill--primary" v-busy="busy" @click="deleteSport(s, i)">
              Delete<span class="visually-hidden"> {{ s.label }}</span>
            </button>
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
          <span class="name">{{ m.label }}</span><span class="meta">{{ m.key }} · {{ m.aggregation }}{{ m.unit ? " · " + m.unit : "" }}</span>
          <span v-if="confirmDelete === `metric:${m.id}`" class="confirm">
            <span>Delete this metric?</span>
            <button v-focus="true" type="button" class="pill" @click="kept = `metric:${m.id}`; confirmDelete = ''">Keep</button>
            <button type="button" class="pill pill--primary" v-busy="busy" @click="deleteMetric(m, i)">
              Delete<span class="visually-hidden"> {{ m.label }}</span>
            </button>
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
  border: 2px solid var(--ink);
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
.meta {
  flex: 1;
  min-width: 8rem;
}
.ban {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: var(--space-3);
  width: 100%;
}
.ban .rb-field {
  flex: 1 1 16rem;
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
.confirm {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  font-weight: 700;
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
  border-bottom: 2px solid var(--ink);
}
.mono {
  font-family: ui-monospace, monospace;
  font-size: 0.75rem;
  overflow-wrap: anywhere;
}
</style>
