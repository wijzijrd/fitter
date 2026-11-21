<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { adminClient } from "../lib/connect";
import { errorMessage } from "../lib/errors";
import { toDate } from "../lib/timestamp";
import type { AdminAction, AdminUser, CustomMetric, CustomSport } from "../gen/fitter/v1/admin_pb";
import { useAuthStore } from "../stores/auth";


const auth = useAuthStore();
type Tab = "users" | "catalog" | "audit";
const tab = ref<Tab>("users");
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
const deleteSport = (s: CustomSport) => {
  if (confirm(`Delete sport "${s.label}"?`)) act(() => adminClient.deleteCustomSport({ id: s.id }));
};
const deleteMetric = (m: CustomMetric) => {
  if (confirm(`Delete metric "${m.label}"?`)) act(() => adminClient.deleteCustomMetric({ id: m.id }));
};

const fmtDate = (ts: Parameters<typeof toDate>[0]) => toDate(ts)?.toLocaleString() ?? "";
const isSelf = (u: AdminUser) => u.id === auth.user?.id;
</script>

<template>
  <main class="admin">
    <h1>Admin</h1>
    <div role="tablist" class="tabs">
      <button v-for="t in (['users', 'catalog', 'audit'] as Tab[])" :key="t" role="tab" type="button"
        class="pill" :class="{ 'pill--primary': tab === t }" :aria-selected="tab === t" @click="tab = t">
        {{ t === "users" ? "Users" : t === "catalog" ? "Catalog" : "Audit" }}
      </button>
    </div>
    <p v-if="error" class="notice" role="alert">{{ error }}</p>

    <section v-if="tab === 'users'">
      <h2>Pending ({{ pending.length }})</h2>
      <p v-if="!pending.length" class="quiet">Nobody waiting.</p>
      <ul class="rows">
        <li v-for="u in pending" :key="u.id">
          <span class="name">{{ u.name }}</span>
          <span class="meta">Joined {{ fmtDate(u.createdAt) }}</span>
          <button type="button" class="pill pill--primary" :disabled="busy" @click="approve(u)">Approve</button>
          <button type="button" class="pill" :disabled="busy" @click="startBan(u)">Ban…</button>
          <form v-if="banning === u.id" class="ban" @submit.prevent="confirmBan(u)">
            <input v-model="banReason" class="input" maxlength="500" required placeholder="Reason" :aria-label="`Ban reason for ${u.name}`" />
            <button type="submit" class="pill" :disabled="busy || !banReason.trim()">Confirm ban</button>
          </form>
        </li>
      </ul>

      <h2>Active ({{ active.length }})</h2>
      <ul class="rows">
        <li v-for="u in active" :key="u.id">
          <span class="name">{{ u.name }}</span>
          <span class="meta">{{ u.role }}</span>
          <template v-if="!isSelf(u)">
            <button v-if="u.role === 'member'" type="button" class="pill" :disabled="busy" @click="setRole(u, 'admin')">Promote</button>
            <button v-else type="button" class="pill" :disabled="busy" @click="setRole(u, 'member')">Demote</button>
            <button v-if="u.role === 'member'" type="button" class="pill" :disabled="busy" @click="startBan(u)">Ban…</button>
          </template>
          <span v-else class="meta">you</span>
          <form v-if="banning === u.id" class="ban" @submit.prevent="confirmBan(u)">
            <input v-model="banReason" class="input" maxlength="500" required placeholder="Reason" :aria-label="`Ban reason for ${u.name}`" />
            <button type="submit" class="pill" :disabled="busy || !banReason.trim()">Confirm ban</button>
          </form>
        </li>
      </ul>

      <h2>Banned ({{ banned.length }})</h2>
      <p v-if="!banned.length" class="quiet">No bans.</p>
      <ul class="rows">
        <li v-for="u in banned" :key="u.id">
          <span class="name">{{ u.name }}</span>
          <span class="meta">{{ u.banReason }} · {{ fmtDate(u.bannedAt) }}</span>
          <button type="button" class="pill" :disabled="busy" @click="unban(u)">Unban</button>
        </li>
      </ul>
    </section>

    <section v-else-if="tab === 'catalog'">
      <h2>Custom sports</h2>
      <ul class="rows">
        <li v-for="s in sports" :key="s.id">
          <span class="name">{{ s.label }}</span><span class="meta">{{ s.key }}</span>
          <button type="button" class="pill" :disabled="busy" @click="deleteSport(s)">Delete</button>
        </li>
      </ul>
      <form class="form" @submit.prevent="createSport">
        <input v-model="sportForm.key" class="input" required placeholder="key (e.g. powerlifting)" aria-label="Sport key" />
        <input v-model="sportForm.label" class="input" required placeholder="Label" aria-label="Sport label" />
        <button type="submit" class="pill pill--primary" :disabled="busy">Add sport</button>
      </form>

      <h2>Custom metrics</h2>
      <ul class="rows">
        <li v-for="m in metrics" :key="m.id">
          <span class="name">{{ m.label }}</span><span class="meta">{{ m.key }} · {{ m.aggregation }}{{ m.unit ? " · " + m.unit : "" }}</span>
          <button type="button" class="pill" :disabled="busy" @click="deleteMetric(m)">Delete</button>
        </li>
      </ul>
      <form class="form" @submit.prevent="createMetric">
        <input v-model="metricForm.key" class="input" required placeholder="key" aria-label="Metric key" />
        <input v-model="metricForm.label" class="input" required placeholder="Label" aria-label="Metric label" />
        <input v-model="metricForm.unit" class="input" placeholder="Unit (kg, reps)" aria-label="Metric unit" />
        <select v-model="metricForm.aggregation" class="input" aria-label="Aggregation">
          <option>sum</option><option>average</option><option>max</option><option>count</option>
        </select>
        <input v-model="metricForm.refs" class="input" placeholder="Sports/groups, comma separated (blank = all)" aria-label="Applicable sports" />
        <label><input v-model="metricForm.nullable" type="checkbox" /> Nullable</label>
        <label><input v-model="metricForm.lowerIsBetter" type="checkbox" /> Lower is better</label>
        <label><input v-model="metricForm.supportsSingleScope" type="checkbox" /> Single scope</label>
        <label><input v-model="metricForm.supportsAggregateScope" type="checkbox" /> Aggregate scope</label>
        <button type="submit" class="pill pill--primary" :disabled="busy">Add metric</button>
      </form>
    </section>

    <section v-else>
      <h2>Audit log</h2>
      <p v-if="!actions.length" class="quiet">Nothing recorded yet.</p>
      <table v-else>
        <thead><tr><th>When</th><th>Admin</th><th>Action</th><th>Target</th><th>Detail</th></tr></thead>
        <tbody>
          <tr v-for="a in actions" :key="a.id">
            <td>{{ fmtDate(a.createdAt) }}</td><td>{{ a.actorName }}</td><td>{{ a.action }}</td>
            <td class="mono">{{ a.targetId }}</td><td>{{ a.detail }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </main>
</template>

<style scoped>
.admin { max-width: 960px; margin: 0 auto; padding: 1.5rem; }
h1 { font: 400 2rem / 1 var(--font-display); text-transform: uppercase; margin: 0 0 1rem; }
h2 { font-size: 1.1rem; margin: 1.5rem 0 0.5rem; }
.tabs { display: flex; gap: 0.5rem; margin-bottom: 1rem; }
.notice { padding: 0.75rem 1rem; border: 2px solid var(--ink); border-radius: 10px; background: var(--cool-blue); font-weight: 700; }
.quiet, .meta { color: var(--ink-muted); }
.rows { list-style: none; margin: 0; padding: 0; }
.rows li { display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem 0.75rem; padding: 0.6rem 0; border-bottom: 1px solid var(--hairline); }
.name { font-weight: 700; }
.meta { flex: 1; min-width: 8rem; }
.ban, .form { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; width: 100%; }
.form { margin-top: 0.75rem; }
table { width: 100%; border-collapse: collapse; font-size: 0.875rem; }
th, td { text-align: left; padding: 0.4rem 0.5rem; border-bottom: 1px solid var(--hairline); vertical-align: top; }
.mono { font-family: ui-monospace, monospace; font-size: 0.75rem; overflow-wrap: anywhere; }
</style>
