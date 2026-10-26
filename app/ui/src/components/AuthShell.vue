<script setup lang="ts">
import poweredByStrava from "../assets/strava/api_logo_pwrdBy_strava_horiz_black.svg";
</script>

<template>
  <main class="auth">
    <!-- Decorative ground: horizontal and vertical dumbbells interlocked on a 148px tile. Corner copies are clipped by the tile and complete each other. -->
    <svg class="weights" aria-hidden="true" focusable="false">
      <defs>
        <g id="dumbbell">
          <rect x="-30" y="-4" width="60" height="8" />
          <rect x="-34" y="-20" width="12" height="40" rx="3" />
          <rect x="22" y="-20" width="12" height="40" rx="3" />
          <rect x="-42" y="-12" width="8" height="24" rx="3" />
          <rect x="34" y="-12" width="8" height="24" rx="3" />
        </g>
        <pattern id="weights-tile" width="148" height="148" patternUnits="userSpaceOnUse">
          <use href="#dumbbell" />
          <use href="#dumbbell" x="148" />
          <use href="#dumbbell" y="148" />
          <use href="#dumbbell" x="148" y="148" />
          <use href="#dumbbell" transform="translate(74 74) rotate(90)" />
        </pattern>
      </defs>
      <rect width="100%" height="100%" fill="url(#weights-tile)" />
    </svg>
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
/* One group (wordmark, bib, attribution) centred at every size; auto margins
   centre it without clipping when a short viewport makes it taller than the screen. */
.auth {
  position: relative;
  overflow: hidden;
  display: flex;
  min-height: 100vh;
  min-height: 100dvh;
  background: var(--paper);
  color: var(--ink);
  font-family: var(--font-body);
}
.weights {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  fill: var(--hairline);
}
.auth-col {
  position: relative;
  width: 100%;
  max-width: 400px;
  margin: auto;
  /* Extra bottom padding lifts the group to the optical centre. */
  padding: 1.5rem 1rem clamp(3rem, 14vh, 7rem);
}
/* Same size as the navbar wordmark: the bib below is the page's hero. */
h1 {
  /* Paper backing keeps the wordmark and Strava's logo off the pattern. */
  display: table;
  margin: 0 0 1rem -0.5rem;
  padding: 0.25rem 0.5rem;
  background: var(--paper);
  font: 400 1.25rem / 1.1 var(--font-display);
  text-transform: uppercase;
}
@media (min-width: 600px) {
  .auth-col {
    max-width: 480px;
  }
}
@media (min-width: 900px) {
  .auth-col {
    max-width: 520px;
  }
}
.attribution {
  display: table;
  margin: 1rem 0 0 -0.5rem;
  padding: 0.5rem;
  background: var(--paper);
}
.attribution img {
  display: block;
}
</style>
