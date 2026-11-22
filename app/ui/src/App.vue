<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useAuthStore } from "./stores/auth";
import { useWhatsNewStore } from "./stores/whatsNew";
import { useSyncStore } from "./stores/sync";
import WhatsNewDialog from "./components/WhatsNewDialog.vue";
import { useRoute, useRouter } from "vue-router";

const auth = useAuthStore();
const whatsNew = useWhatsNewStore();
const activitySync = useSyncStore();
const router = useRouter();
const route = useRoute();

// Under 720px the links live behind a menu button; the same <nav> is reused, so there is one set of links to keep in order.
const open = ref(false);
const bar = ref<HTMLElement | null>(null);
const toggle = ref<HTMLButtonElement | null>(null);

function handleLogout(): void {
  open.value = false;
  auth.logout();
  router.push({ name: "login" });
}

function close(returnFocus = false): void {
  open.value = false;
  if (returnFocus) nextTick(() => toggle.value?.focus());
}

function onKey(ev: KeyboardEvent): void {
  if (open.value && ev.key === "Escape") close(true);
}
function onPointer(ev: PointerEvent): void {
  if (open.value && bar.value && !bar.value.contains(ev.target as Node)) close();
}

watch(() => route.fullPath, () => close());
// The router guard loads the user after sign-in; once it is there, offer the splash if it is unseen.
watch(() => auth.user, () => whatsNew.offer(), { immediate: true });

// Pull the member's new Strava activities on load, when they come back to the tab, and every five
// minutes while it stays open. Pending members have no campaigns to see them in.
function syncNow(): void {
  if (auth.user && !auth.isPending && document.visibilityState === "visible") activitySync.sync();
}
let syncTimer: ReturnType<typeof setInterval> | undefined;
watch(() => auth.user?.id, syncNow, { immediate: true });
onMounted(() => {
  document.addEventListener("keydown", onKey);
  document.addEventListener("pointerdown", onPointer);
  document.addEventListener("visibilitychange", syncNow);
  syncTimer = setInterval(syncNow, 5 * 60 * 1000);
});
onBeforeUnmount(() => {
  document.removeEventListener("keydown", onKey);
  document.removeEventListener("pointerdown", onPointer);
  document.removeEventListener("visibilitychange", syncNow);
  clearInterval(syncTimer);
});
</script>

<template>
  <div ref="bar" class="navbar" :class="{ 'is-open': open }" v-if="auth.isAuthenticated">
    <router-link to="/" class="wordmark">Fitter</router-link>
    <button
      ref="toggle"
      type="button"
      class="menu-btn"
      :aria-expanded="open"
      aria-controls="site-nav"
      @click="open = !open"
    >
      <svg viewBox="0 0 20 20" aria-hidden="true">
        <path v-if="!open" d="M3 6h14M3 10h14M3 14h14" />
        <path v-else d="M5 5l10 10M15 5L5 15" />
      </svg>
      <span class="visually-hidden">{{ open ? "Close menu" : "Open menu" }}</span>
    </button>
    <nav id="site-nav" aria-label="Main">
      <router-link to="/" :class="{ 'router-link-active': String($route.name).startsWith('campaign') }">Campaigns</router-link>
      <template v-if="!auth.isPending">
        <router-link to="/goals">Goals</router-link>
        <router-link to="/activities/new">Log activity</router-link>
        <router-link v-if="auth.isAdmin" to="/admin">Admin</router-link>
      </template>
      <router-link v-if="auth.user" to="/settings" class="who" :aria-label="`Settings, signed in as ${auth.user.name}`">{{ auth.user.name }}</router-link>
      <button class="logout" @click="handleLogout">Log out</button>
    </nav>
  </div>
  <router-view />
  <WhatsNewDialog v-if="whatsNew.open" @close="whatsNew.dismiss()" />
</template>
