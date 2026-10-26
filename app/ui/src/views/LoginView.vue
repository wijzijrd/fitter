<script setup lang="ts">
import connectWithStrava from "../assets/strava/btn_strava_connect_with_orange.svg";
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL as string;
const loginUrl = `${apiBaseUrl}/auth/strava/login`;
</script>

<template>
  <div class="container" style="max-width: 420px; text-align: center; margin-top: 4rem">
    <h1>Fitter</h1>
    <p class="muted">Track activities and campaigns with your friends.</p>
    <!-- Strava's official button, unmodified, per their brand guidelines. -->
    <a class="strava-connect" :href="loginUrl">
      <img :src="connectWithStrava" alt="Connect with Strava" width="237" height="48" />
    </a>
  </div>
</template>

<style scoped>
.strava-connect {
  display: inline-block;
  margin-top: 0.5rem;
  border-radius: 6px;
}
.strava-connect img {
  display: block;
}
.strava-connect:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: 3px;
}
</style>
