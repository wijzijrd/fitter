<script setup lang="ts">
import { CURRENT, kindLabels, kindOf, releases } from "../lib/releases";

const day = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", year: "numeric" });
const when = (date: string) => day.format(new Date(`${date}T00:00:00`));
</script>

<template>
  <main class="st">
    <router-link to="/settings" class="back">
      <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M16 10H5M9 5l-5 5 5 5" /></svg>Settings
    </router-link>
    <h1 id="st-title" tabindex="-1">Release notes</h1>
    <p class="meta">You're on version {{ CURRENT }}.</p>
    <section v-for="r in releases" :key="r.version" class="rel" :aria-labelledby="`rel-${r.version}`">
      <h2 :id="`rel-${r.version}`">{{ r.version }}</h2>
      <p class="meta">{{ kindLabels[kindOf(r.version)] }} · {{ when(r.date) }}</p>
      <ul>
        <li v-for="n in r.notes" :key="n.title">
          <b>{{ n.title }}</b>
          <span>{{ n.body }}</span>
        </li>
      </ul>
    </section>
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
  margin: var(--space-3) 0 var(--space-2);
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}
.meta {
  margin: 0;
  color: var(--ink-muted);
  font-size: 0.875rem;
}
.rel {
  margin-top: var(--space-6);
  padding-top: var(--space-5);
  border-top: 1px solid var(--hairline);
}
.rel h2 {
  margin: 0 0 var(--space-1);
  font: 400 1.125rem/1.2 var(--font-display);
}
ul {
  display: grid;
  gap: var(--space-3);
  margin: var(--space-3) 0 0;
  padding: 0;
  list-style: none;
}
li {
  display: grid;
  gap: var(--space-1);
}
li span {
  color: var(--ink-muted);
  font-size: 0.875rem;
  line-height: 1.45;
}
.back svg {
  width: 1rem;
  height: 1rem;
  flex: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
