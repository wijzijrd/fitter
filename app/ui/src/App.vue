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
    <router-link to="/" class="wordmark">Fitter</router-link>
    <nav>
      <router-link to="/" :class="{ 'router-link-active': String($route.name).startsWith('campaign') }">Campaigns</router-link>
      <router-link to="/goals">Goals</router-link>
      <span class="who" v-if="auth.user">{{ auth.user.name }}</span>
      <button class="logout" @click="handleLogout">Log out</button>
    </nav>
  </div>
  <router-view />
</template>
