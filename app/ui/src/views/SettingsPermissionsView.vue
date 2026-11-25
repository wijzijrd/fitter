<script setup lang="ts">
import { ref } from "vue";
import { useAuthStore } from "../stores/auth";
import { errorMessage } from "../lib/errors";
import { useWhatsNewStore } from "../stores/whatsNew";
import type { FeedSettings } from "../gen/fitter/v1/user_pb";

const auth = useAuthStore();
const whatsNew = useWhatsNewStore();
const error = ref("");

// Each choice saves as soon as it's toggled. The checkboxes read the saved value, so a failed
// save leaves them showing what is actually stored.
const options: { key: keyof FeedSettings & string; title: string; hint: string }[] = [
  { key: "showPace", title: "Pace and speed", hint: "Pace for runs and walks, speed for rides." },
  { key: "showPower", title: "Power", hint: "Average watts, when the ride has a power meter." },
  { key: "showEffort", title: "Effort", hint: "Strava's relative effort score." },
  { key: "showLifts", title: "Exercises", hint: "The exercises of a logged gym or circuit session." },
];

async function toggle(key: string, on: boolean): Promise<void> {
  error.value = "";
  try {
    await auth.saveFeedSettings({ ...auth.feedSettings, [key]: on } as FeedSettings);
  } catch (err) {
    error.value = errorMessage(err, "That wasn't saved. Try again.");
  }
}
</script>

<template>
  <main class="st">
    <router-link to="/settings" class="back">
      <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M16 10H5M9 5l-5 5 5 5" /></svg>Settings
    </router-link>
    <h1 id="st-title" tabindex="-1">App permissions</h1>
    <fieldset class="rb-field details">
      <legend>Feed details</legend>
      <p class="meta">What shows under each activity in a campaign's feed. Only your own view changes: it doesn't change what anyone sees of you.</p>
      <label v-for="o in options" :key="o.key" class="choice">
        <input type="checkbox" :checked="auth.feedSettings[o.key] as boolean" @change="toggle(o.key, ($event.target as HTMLInputElement).checked)" />
        <span>
          <b>{{ o.title }}</b>
          <small>{{ o.hint }}</small>
        </span>
      </label>
      <p v-if="error" class="field-error" role="alert">{{ error }}</p>
    </fieldset>
    <section class="rb-field news" aria-labelledby="st-news">
      <h2 id="st-news">What's new</h2>
      <p class="meta">The changes since the last time you read them.</p>
      <button type="button" class="pill" @click="whatsNew.show()">Show what's new</button>
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
  margin: var(--space-3) 0 var(--space-6);
  font: 400 clamp(1.625rem, 6.5vw, 2.5rem) / 1.05 var(--font-display);
  text-transform: uppercase;
}
.details {
  border: 0;
  padding: 0;
  margin: 0;
  gap: var(--space-3);
}
.details legend {
  margin-bottom: var(--space-2);
  font: 400 0.875rem/1.3 var(--font-display);
  text-transform: uppercase;
}
.meta {
  margin: 0 0 var(--space-3);
  color: var(--ink-muted);
  font-size: 0.875rem;
}
.news {
  margin-top: var(--space-6);
}
.news h2 {
  margin: 0 0 var(--space-2);
  font: 400 0.875rem/1.3 var(--font-display);
  text-transform: uppercase;
}
.news .pill {
  justify-self: start;
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
