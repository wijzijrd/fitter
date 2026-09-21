<script setup lang="ts">
import { useAuthStore } from "./stores/auth";
import { useRouter } from "vue-router";
import { Button } from "@/components/ui/button";

const auth = useAuthStore();
const router = useRouter();

function handleLogout(): void {
  auth.logout();
  router.push({ name: "login" });
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
      <Button variant="outline" size="sm" @click="handleLogout">Log out</Button>
    </nav>
  </div>
  <router-view />
</template>
