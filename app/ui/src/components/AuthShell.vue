<script setup lang="ts">
import poweredByStrava from "../assets/strava/api_logo_pwrdBy_strava_horiz_black.svg";
</script>

<template>
  <main class="auth">
    <div class="auth-col">
      <h1>Fitter</h1>
      <slot />
      <footer class="attribution">
        <img :src="poweredByStrava" alt="Powered by Strava" width="158" height="16" />
      </footer>
    </div>
  </main>
</template>

<style scoped>
.auth {
  min-height: 100vh;
  background: var(--paper);
  color: var(--ink);
  font-family: var(--font-body);
}
.auth-col {
  max-width: 400px;
  margin: 0 auto;
  padding: clamp(1.5rem, 8vh, 4.5rem) 1rem 2rem;
}
/* Same size as the navbar wordmark: the bib below is the page's hero. */
h1 {
  margin: 0 0 1rem;
  font: 400 1.25rem / 1.1 var(--font-display);
  text-transform: uppercase;
}
@media (min-width: 900px) {
  .auth {
    display: grid;
    align-items: center;
  }
  .auth-col {
    width: 100%;
    max-width: 460px;
    padding-block: 2rem;
  }
}
.attribution {
  margin-top: 1.5rem;
}
.attribution img {
  display: block;
}
</style>
