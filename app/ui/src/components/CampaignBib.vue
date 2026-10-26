<script setup lang="ts">
import { RouterLink } from "vue-router";

defineProps<{
  band: [string, string];
  numeral?: string;
  // numeralLabel is what a screen reader hears instead of the bare numeral ("3rd of 6").
  numeralLabel?: string;
  value?: string;
  subs?: string[];
  tab?: string;
  finisher?: boolean;
  // blue sets a Cool Blue band without the FINISHER meaning (e.g. when Tangerine would compete with a nearby CTA).
  blue?: boolean;
  pending?: boolean;
  // to turns the campaign name into a link whose hit area covers the whole bib.
  to?: string;
}>();
</script>

<template>
  <article class="rbib" :class="{ 'rbib--link': to }">
    <div class="band" :class="{ 'band--blue': finisher || blue }">
      <RouterLink v-if="to" :to="to" class="name">{{ band[0] }}</RouterLink>
      <span v-else class="name">{{ band[0] }}</span>
      <span>{{ band[1] }}</span>
    </div>
    <div class="main">
      <slot>
        <span v-if="numeral" class="numeral" :class="{ 'numeral--long': numeral.length > 1, pending }" aria-hidden="true">{{ numeral }}</span>
        <span v-if="numeralLabel" class="visually-hidden">{{ numeralLabel }}</span>
        <span class="value">
          <b v-if="value" :class="{ 'value--long': value.length > 9 }">{{ value }}</b>
          <small v-for="line in subs" :key="line">{{ line }}</small>
        </span>
      </slot>
    </div>
    <div v-if="tab || $slots.tab" class="tab">
      <slot name="tab">{{ tab }}</slot>
    </div>
  </article>
</template>

<style scoped>
.rbib {
  position: relative;
  display: flex;
  flex-direction: column;
  border: 2px solid var(--ink);
  border-radius: 10px;
  overflow: hidden;
  background: var(--paper);
  color: var(--ink);
  font-family: var(--font-body);
  transition: background-color 160ms var(--ease-out-expo);
}
.rbib--link:has(.name:hover) {
  background: var(--cool-blue-soft);
}

.band {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.3rem 0.75rem;
  background: var(--tangerine);
  font: 400 0.75rem/1.3 var(--font-display);
  text-transform: uppercase;
  white-space: nowrap;
}
.band > * {
  overflow: hidden;
  text-overflow: ellipsis;
}
.band > :last-child {
  flex: none;
}
.band--blue {
  background: var(--cool-blue);
}
.name {
  color: inherit;
}
.name:focus-visible {
  outline: none;
}
/* Stretched link: the campaign name carries the accessible name, the bib carries the hit area. */
.rbib--link .name::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 8px;
}
.rbib--link .name:focus-visible::after {
  outline: 3px solid var(--ink);
  outline-offset: 3px;
}

.main {
  flex: 1;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 7rem;
  padding: 0.6rem 0.9rem 0.55rem;
}
.numeral {
  font: 400 clamp(4rem, 20vw, 5.5rem) / 0.85 var(--font-display);
  font-variant-numeric: tabular-nums;
}
.numeral--long {
  font-size: clamp(3rem, 15vw, 4rem);
}
.numeral.pending {
  color: var(--hairline);
}
.value {
  /* Stays right-aligned when there is no numeral. */
  margin-left: auto;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  min-width: 0;
  text-align: right;
}
.value b {
  font: 400 1.625rem/1.1 var(--font-display);
  font-variant-numeric: tabular-nums;
}
.value b.value--long {
  font-size: 1.125rem;
}
.value small {
  color: var(--ink-muted);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
}

.tab {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  border-top: 2px dashed var(--ink);
  background: var(--cool-blue);
  padding: 0.5rem 0.9rem;
  font-weight: 700;
  font-size: 0.9375rem;
  font-variant-numeric: tabular-nums;
}
/* Controls in the tab sit above the stretched link. */
.tab :deep(button) {
  position: relative;
  z-index: 1;
}
</style>
