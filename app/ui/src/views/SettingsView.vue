<script setup lang="ts">
const sections = [
  { to: "/settings/permissions", title: "App permissions", hint: "Which details show in campaign feeds, and what's new." },
  { to: "/settings/feedback", title: "Developer feedback", hint: "Report a bug or suggest an idea." },
];
</script>

<template>
  <main class="st">
    <h1 id="st-title" tabindex="-1">Settings</h1>
    <nav aria-labelledby="st-title">
      <ul>
        <li v-for="s in sections" :key="s.to">
          <router-link :to="s.to" class="choice">
            <span>
              <b>{{ s.title }}</b>
              <small>{{ s.hint }}</small>
            </span>
            <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 10h11M11 5l5 5-5 5" /></svg>
          </router-link>
        </li>
      </ul>
    </nav>
  </main>
</template>

<style scoped>
.st {
  max-width: 560px;
  margin: 0 auto;
  padding: var(--space-5) var(--gutter) var(--space-8);
  font-family: var(--font-body);
  color: var(--ink);
}
h1 {
  margin: var(--space-3) 0 var(--space-6);
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}
ul {
  display: grid;
  gap: var(--space-3);
  margin: 0;
  padding: 0;
  list-style: none;
}
.choice {
  justify-content: space-between;
  align-items: center;
  min-height: 2.75rem;
  text-decoration: none;
}
.choice small {
  color: var(--ink-muted);
  font-size: 0.875rem;
}
.choice:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: 2px;
}
svg {
  flex: none;
  width: 1.25rem;
  height: 1.25rem;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
