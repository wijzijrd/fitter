<script setup lang="ts">
import { useAuthStore } from "./stores/auth";
import { useRouter } from "vue-router";

const auth = useAuthStore();
const router = useRouter();

function handleLogout(): void {
  auth.logout();
  router.push({ name: "login" });
}
</script>

<template>
  <div class="navbar" v-if="auth.isAuthenticated">
    <strong>Fitter</strong>
    <nav>
      <router-link to="/">Campaigns</router-link>
      <router-link to="/goals">Goals</router-link>
      <span class="muted" v-if="auth.user">{{ auth.user.name }}</span>
      <button class="btn secondary" @click="handleLogout">Log out</button>
    </nav>
  </div>
  <router-view />
</template>
