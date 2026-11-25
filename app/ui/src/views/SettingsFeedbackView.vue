<script setup lang="ts">
import { computed, ref } from "vue";
import { Code, ConnectError } from "@connectrpc/connect";
import { useAuthStore } from "../stores/auth";
import { userClient } from "../lib/connect";
import { errorMessage } from "../lib/errors";
import { CURRENT } from "../lib/releases";

const auth = useAuthStore();

// Feedback is emailed to the developer with Reply-To set to the member's address, so the address
// is verified first: say why, then enter it, get a 6-digit code, type the code.
const started = ref(false);
const changing = ref(false);
const verified = computed(() => !!auth.user?.email && !changing.value);
const email = ref(auth.user?.email ?? "");
const sentTo = ref("");
const code = ref("");

const kinds = [
  { value: "bug", label: "Bug" },
  { value: "idea", label: "Idea" },
  { value: "other", label: "Other" },
];
const kind = ref("bug");
const message = ref("");
const status = ref("");

const busy = ref(false);
const error = ref("");
// Focus follows the swap between steps, but not on first load (no keyboard popping up unasked).
const moved = ref(false);

// The server's codes say which step went wrong; anything else gets the shared copy.
function failure(err: unknown, own: Partial<Record<Code, string>>, fallback: string): string {
  return own[ConnectError.from(err).code] ?? errorMessage(err, fallback);
}

async function run(fn: () => Promise<void>, own: Partial<Record<Code, string>>, fallback: string): Promise<void> {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    await fn();
    moved.value = true;
  } catch (err) {
    error.value = failure(err, own, fallback);
  } finally {
    busy.value = false;
  }
}

const sendCode = () =>
  run(
    async () => {
      await userClient.sendEmailCode({ email: email.value.trim() });
      sentTo.value = email.value.trim();
      code.value = "";
    },
    {
      [Code.InvalidArgument]: "Enter an email address like name@example.com.",
      [Code.ResourceExhausted]: "A code was sent less than a minute ago. Wait a moment, then try again.",
      [Code.Unavailable]: "The code couldn't be sent. Try again in a minute.",
    },
    "The code couldn't be sent. Try again.",
  );

const verify = () =>
  run(
    async () => {
      await auth.verifyEmail(code.value.trim());
      changing.value = false;
      sentTo.value = "";
    },
    {
      [Code.InvalidArgument]: "That code doesn't match. Check the email and try again.",
      [Code.FailedPrecondition]: "That code has expired or had too many tries. Send a new one.",
    },
    "That code wasn't checked. Try again.",
  );

const submit = () =>
  run(
    async () => {
      status.value = "";
      await userClient.submitFeedback({ kind: kind.value, message: message.value, release: CURRENT });
      message.value = "";
      status.value = "Thanks, it's sent.";
    },
    {
      [Code.InvalidArgument]: "Write a message of up to 4,000 characters.",
      [Code.FailedPrecondition]: "Verify your email first.",
      [Code.Unavailable]: "It couldn't be sent. Try again in a minute.",
    },
    "It wasn't sent. Try again.",
  );

function start(): void {
  started.value = true;
  moved.value = true;
}

function restart(): void {
  started.value = true;
  changing.value = true;
  sentTo.value = "";
  error.value = "";
  status.value = "";
  moved.value = true;
}
</script>

<template>
  <main class="st">
    <router-link to="/settings" class="back">
      <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M16 10H5M9 5l-5 5 5 5" /></svg>Settings
    </router-link>
    <h1 id="st-title" tabindex="-1">Developer feedback</h1>

    <template v-if="!verified">
      <section v-if="!started" class="gate" aria-labelledby="fb-gate">
        <h2 id="fb-gate">Verify your email to send feedback</h2>
        <p class="meta">
          Bug reports and ideas go straight to the developer by email. To send one, confirm your email address first, so a reply can reach
          you. It's only used for feedback.
        </p>
        <button type="button" class="pill pill--primary" @click="start">Verify email</button>
      </section>
      <form v-else-if="!sentTo" class="rb-field" novalidate @submit.prevent="sendCode">
        <p class="step">Step 1 of 2</p>
        <label for="fb-email">Your email</label>
        <small id="fb-email-hint">We'll send a 6-digit code to it.</small>
        <input
          id="fb-email"
          v-model="email"
          v-focus="moved"
          type="email"
          class="input"
          autocomplete="email"
          maxlength="254"
          :aria-invalid="!!error"
          :aria-describedby="error ? 'fb-email-hint fb-error' : 'fb-email-hint'"
        />
        <p v-if="error" id="fb-error" class="field-error" role="alert">{{ error }}</p>
        <button type="submit" class="pill pill--primary" v-busy="busy">{{ busy ? "Sending…" : "Send code" }}</button>
      </form>
      <form v-else class="rb-field" novalidate @submit.prevent="verify">
        <p class="step">Step 2 of 2</p>
        <label for="fb-code">6-digit code</label>
        <small id="fb-code-hint">We sent it to {{ sentTo }}. It works for 15 minutes.</small>
        <input
          id="fb-code"
          v-model="code"
          v-focus="true"
          class="input code"
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="6"
          :aria-invalid="!!error"
          :aria-describedby="error ? 'fb-code-hint fb-error' : 'fb-code-hint'"
        />
        <p v-if="error" id="fb-error" class="field-error" role="alert">{{ error }}</p>
        <div class="actions">
          <button type="submit" class="pill pill--primary" v-busy="busy">{{ busy ? "Checking…" : "Verify" }}</button>
          <button type="button" class="pill pill--quiet" @click="restart">Send a new code</button>
        </div>
      </form>
    </template>

    <form v-else class="fb" novalidate @submit.prevent="submit">
      <div class="reply">
        <p class="meta">Replies go to <b>{{ auth.user?.email }}</b>.</p>
        <button type="button" class="pill pill--quiet" @click="restart">Change email</button>
      </div>
      <fieldset class="rb-field kinds">
        <legend>Kind</legend>
        <div class="pills">
          <label v-for="k in kinds" :key="k.value" class="choice choice--pill">
            <input v-model="kind" type="radio" name="fb-kind" :value="k.value" />{{ k.label }}
          </label>
        </div>
      </fieldset>
      <div class="rb-field">
        <label for="fb-message">Message</label>
        <textarea
          id="fb-message"
          v-model="message"
          v-focus="moved"
          class="input"
          rows="6"
          maxlength="4000"
          placeholder="What happened, or what would make Fitter better?"
          :aria-invalid="!!error"
          :aria-describedby="error ? 'fb-error' : undefined"
        />
        <p v-if="error" id="fb-error" class="field-error" role="alert">{{ error }}</p>
      </div>
      <button type="submit" class="pill pill--primary" v-busy="busy">{{ busy ? "Sending…" : "Send feedback" }}</button>
      <p class="meta sent" role="status">{{ status }}</p>
    </form>
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
.meta {
  margin: 0 0 var(--space-3);
  color: var(--ink-muted);
  font-size: 0.875rem;
  overflow-wrap: anywhere;
}
.meta b {
  color: var(--ink);
}
.rb-field .pill,
.fb > .pill {
  justify-self: start;
  align-self: flex-start;
}
.gate {
  display: grid;
  gap: var(--space-3);
  padding: var(--space-5) var(--bib-inset);
  border: 2px solid var(--line);
  border-radius: 10px;
}
.gate h2 {
  margin: 0;
  font: 400 1.125rem/1.2 var(--font-display);
  text-transform: uppercase;
}
.gate .meta {
  margin: 0;
}
.gate .pill {
  justify-self: start;
}
.step {
  margin: 0;
  color: var(--ink-muted);
  font: 400 0.75rem/1.2 var(--font-display);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.code {
  max-width: 10rem;
  letter-spacing: 0.3em;
}
.actions,
.pills {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.reply {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  margin-bottom: var(--space-5);
}
.reply .meta {
  margin: 0;
}
.kinds {
  border: 0;
  padding: 0;
}
textarea {
  resize: vertical;
}
.sent {
  margin-top: var(--space-3);
  color: var(--ink);
  font-weight: 700;
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
