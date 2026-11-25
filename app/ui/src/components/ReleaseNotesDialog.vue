<script setup lang="ts">
import { onMounted, ref } from "vue";
import { kindLabels, kindOf, type Release } from "../lib/releases";

defineProps<{ releases: Release[] }>();
const emit = defineEmits<{ close: [] }>();
const dlg = ref<HTMLDialogElement | null>(null);

// A native modal <dialog> gives the focus trap, Escape and focus return; its `close` event
// covers both the button and Escape. Only the list scrolls: if the whole dialog did, focusing
// Got it (last in the dialog) on open would scroll it to the bottom.
onMounted(() => dlg.value?.showModal());
</script>

<template>
  <dialog ref="dlg" class="wn" aria-labelledby="wn-title" @close="emit('close')">
    <h2 id="wn-title">Release notes</h2>
    <div class="items" tabindex="0" role="region" aria-label="Changes">
      <section v-for="r in releases" :key="r.version">
        <h3>{{ r.version }}</h3>
        <p class="kind">{{ kindLabels[kindOf(r.version)] }}</p>
        <ul>
          <li v-for="n in r.notes" :key="n.title">
            <b>{{ n.title }}</b>
            <span>{{ n.body }}</span>
          </li>
        </ul>
      </section>
    </div>
    <form method="dialog">
      <!-- Closing first saves the version and removes the modal before the page changes. -->
      <router-link to="/settings/release-notes" class="all" @click="dlg?.close()">All release notes</router-link>
      <button class="pill pill--primary" autofocus>Got it</button>
    </form>
  </dialog>
</template>

<style scoped>
.wn {
  width: min(32rem, calc(100vw - 2 * var(--gutter)));
  max-height: min(40rem, calc(100dvh - 2 * var(--gutter)));
  padding: var(--space-5) var(--bib-inset);
  border: 2px solid var(--line);
  border-radius: 10px;
  background: var(--paper);
  color: var(--ink);
  font-family: var(--font-body);
  overflow: hidden;
}
/* Only while open: display on a closed <dialog> would override its hiding. */
.wn[open] {
  display: flex;
  flex-direction: column;
}
.wn::backdrop {
  background: rgb(0 0 0 / 0.45);
}
h2 {
  flex: none;
  margin: 0 0 var(--space-4);
  font: 400 clamp(1.375rem, 5vw, 1.75rem) / 1.1 var(--font-display);
  text-transform: uppercase;
}
.items {
  flex: 1 1 auto;
  display: grid;
  align-content: start;
  gap: var(--space-5);
  min-height: 0;
  overflow-y: auto;
  margin: 0 calc(-1 * var(--space-2));
  padding: 0 var(--space-2);
  list-style: none;
  overscroll-behavior: contain;
}
.items:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: 2px;
}
.items h3 {
  margin: 0;
  font: 400 1rem/1.2 var(--font-display);
}
.kind {
  margin: var(--space-1) 0 var(--space-2);
  color: var(--ink-muted);
  font-size: 0.8125rem;
}
.items ul {
  margin: 0;
  padding: 0;
  list-style: none;
}
.items li {
  display: grid;
  gap: var(--space-1);
  padding: var(--space-3) 0;
  border-top: 1px solid var(--hairline);
}
.items li:first-child {
  border-top: 0;
  padding-top: 0;
}
.items span {
  color: var(--ink-muted);
  font-size: 0.875rem;
  line-height: 1.45;
}
form {
  display: flex;
  flex: none;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-top: var(--space-3);
  padding-top: var(--space-4);
  border-top: 1px solid var(--hairline);
}
.all {
  display: inline-flex;
  align-items: center;
  min-height: 2.75rem;
  color: var(--ink);
  font-size: 0.875rem;
  font-weight: 600;
  text-decoration: underline;
  text-underline-offset: 3px;
}
</style>
