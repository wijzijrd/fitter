<script setup lang="ts">
const apiBaseUrl = import.meta.env.VITE_API_BASE_URL as string;
const loginUrl = `${apiBaseUrl}/auth/strava/login`;
</script>

<template>
  <div class="container" style="max-width: 420px; text-align: center; margin-top: 4rem">
    <h1>Fitter</h1>
    <p class="muted">Track activities and campaigns with your friends.</p>
    <a class="btn" :href="loginUrl">Log in with Strava</a>
  </div>
</template>
