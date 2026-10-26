<script setup lang="ts">
import poweredByStrava from "../assets/strava/api_logo_pwrdBy_strava_horiz_black.svg";
</script>

<template>
  <main class="auth">
    <!-- Decorative ground: horizontal and vertical dumbbells interlocked on a 148px grid, alternating running shoes and bicycles in the big gaps, on a 296px double tile. Corner copies are clipped by the tile and complete each other. -->
    <svg class="weights" aria-hidden="true" focusable="false">
      <defs>
        <g id="dumbbell">
          <rect x="-30" y="-4" width="60" height="8" />
          <rect x="-34" y="-20" width="12" height="40" rx="3" />
          <rect x="22" y="-20" width="12" height="40" rx="3" />
          <rect x="-42" y="-12" width="8" height="24" rx="3" />
          <rect x="34" y="-12" width="8" height="24" rx="3" />
        </g>
        <!-- Shoe from SVG Repo (sports-shoes-2), 512 viewBox, centred on the origin. -->
        <g id="shoe" transform="scale(0.15) translate(-256 -256)">
          <path d="M264.141,416.896L14.646,205.121L0,222.387l249.53,211.791l0.036,0.032 c28.359,23.865,64.362,37,101.444,37.008h159.915v-22.639H351.01C319.297,448.578,288.402,437.326,264.141,416.896z" />
          <path d="M351.01,425.948c0,0,63.721,0,120.309,0c56.587,0,48.101-56.591,11.318-65.074 c-24.818-5.728-56.682-18.895-56.682-18.895c-12.577-4.194-22.875-13.389-28.442-25.4c0,0-1.377-2.92-3.688-7.834l-46.194,13.966 c-5.048,1.528-10.381-1.322-11.908-6.369c-1.519-5.048,1.326-10.382,6.374-11.901l43.528-13.167 c-3.743-7.975-8.083-17.241-12.656-27.021l-43.171,13.064c-5.049,1.528-10.382-1.345-11.909-6.377 c-1.531-5.049,1.338-10.382,6.373-11.908l40.529-12.257c-4.312-9.258-8.601-18.476-12.574-27.046l-40.255,12.185 c-5.049,1.52-10.382-1.33-11.912-6.377c-1.515-5.049,1.329-10.382,6.378-11.901l37.708-11.41 c-5.405-11.766-9.326-20.486-10.37-23.192c-4.538-11.798-14.504-44.525-37.245-26.334c-37.391,29.934-105.602,21.158-123.212-11.56 c-13.202-24.522-5.654-50.926,0-71.674c4.526-16.584-18.868-42.443-35.837-19.805C125.464,65.66,24.296,185.458,24.296,185.458 L278.72,399.575C298.961,416.611,324.559,425.948,351.01,425.948z" />
        </g>
        <g id="bicycle" transform="scale(0.8) translate(0 -4)" style="fill: none; stroke: var(--hairline); stroke-width: 5; stroke-linecap: round; stroke-linejoin: round">
          <circle cx="-30" cy="10" r="19" />
          <circle cx="30" cy="10" r="19" />
          <path d="M-30 10 L-6 10 L-12 -12 L22 -14 L30 10 M-6 10 L22 -14 M-30 10 L-12 -12 M22 -14 L20 -21 L28 -21 M-18 -16 L-6 -16" />
          <circle cx="-6" cy="10" r="4.5" style="fill: var(--hairline)" />
        </g>
        <pattern id="weights-tile" width="296" height="148" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
          <use href="#dumbbell" />
          <use href="#dumbbell" x="148" />
          <use href="#dumbbell" x="296" />
          <use href="#dumbbell" y="148" />
          <use href="#dumbbell" x="148" y="148" />
          <use href="#dumbbell" x="296" y="148" />
          <use href="#dumbbell" transform="translate(74 74) rotate(90)" />
          <use href="#dumbbell" transform="translate(222 74) rotate(90)" />
          <use href="#shoe" transform="translate(0 74) rotate(-45)" />
          <use href="#shoe" transform="translate(296 74) rotate(-45)" />
          <use href="#bicycle" transform="translate(148 74) rotate(-45)" />
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
