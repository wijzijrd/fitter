<script setup lang="ts">
import { onMounted, ref } from "vue";
import { whatsNewItems } from "../lib/whatsNew";

const emit = defineEmits<{ close: [] }>();
const dlg = ref<HTMLDialogElement | null>(null);

// A native modal <dialog> gives the focus trap, Escape and focus return; its `close` event
// covers both the button and Escape.
onMounted(() => dlg.value?.showModal());
</script>

<template>
  <dialog ref="dlg" class="wn" aria-labelledby="wn-title" @close="emit('close')">
    <h2 id="wn-title">What's new</h2>
    <ol class="items">
      <li v-for="item in whatsNewItems" :key="item.title">
        <b>{{ item.title }}</b>
        <span>{{ item.body }}</span>
      </li>
    </ol>
    <form method="dialog">
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
  overflow-y: auto;
}
.wn::backdrop {
  background: rgb(0 0 0 / 0.45);
}
h2 {
  margin: 0 0 var(--space-4);
  font: 400 clamp(1.375rem, 5vw, 1.75rem) / 1.1 var(--font-display);
  text-transform: uppercase;
}
.items {
  margin: 0 0 var(--space-5);
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
  justify-content: flex-end;
}
</style>
