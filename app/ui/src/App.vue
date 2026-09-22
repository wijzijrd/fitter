<script setup lang="ts">
import { ref } from "vue";
import { useAuthStore } from "./stores/auth";
import { useRouter } from "vue-router";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { THEME_STORAGE_KEY } from "./lib/theme";

const auth = useAuthStore();
const router = useRouter();

function handleLogout(): void {
  auth.logout();
  router.push({ name: "login" });
}

// Dark mode toggle: main.ts already applied the initial `.dark` class
// before mount (from localStorage, falling back to prefers-color-scheme),
// so read the class here to seed the switch's initial state rather than
// re-deriving it.
const isDark = ref(document.documentElement.classList.contains("dark"));

function handleThemeToggle(checked: boolean): void {
  document.documentElement.classList.toggle("dark", checked);
  try {
    localStorage.setItem(THEME_STORAGE_KEY, checked ? "dark" : "light");
  } catch {
    // localStorage can throw (private browsing, etc.) — the toggle still
    // works for this page load, it just won't persist across reloads.
  }
}
</script>

<template>
  <div
    v-if="auth.isAuthenticated"
    class="flex items-center justify-between border-b border-border bg-card px-6 py-4"
  >
    <strong class="text-lg">Fitter</strong>
    <nav class="flex items-center gap-5">
      <router-link to="/" class="text-sm font-medium hover:underline">Campaigns</router-link>
      <router-link to="/goals" class="text-sm font-medium hover:underline">Goals</router-link>
      <span v-if="auth.user" class="text-sm text-muted-foreground">{{ auth.user.name }}</span>
      <div class="flex items-center gap-2">
        <Label for="dark-mode-toggle" class="text-sm text-muted-foreground">Dark mode</Label>
        <Switch id="dark-mode-toggle" v-model="isDark" @update:model-value="handleThemeToggle" />
      </div>
      <Button variant="outline" size="sm" @click="handleLogout">Log out</Button>
    </nav>
  </div>
  <router-view />
</template>
