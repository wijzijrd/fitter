<script setup lang="ts">
import { Code, ConnectError } from "@connectrpc/connect";
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { takeReturnTo } from "../lib/returnTo";
import AuthShell from "../components/AuthShell.vue";
import CampaignBib from "../components/CampaignBib.vue";

const router = useRouter();
const auth = useAuthStore();
const error = ref("");

onMounted(async () => {
  const hash = new URLSearchParams(window.location.hash.replace(/^#/, ""));
  const token = hash.get("token");
  // Keep the JWT out of the address bar and history, whichever way this ends.
  router.replace({ name: "auth-callback" });
  if (!token) {
    error.value = "Strava sent you back without a sign-in. Try connecting again.";
    return;
  }
  auth.completeLogin(token);
  try {
    await auth.fetchCurrentUser();
    router.replace(takeReturnTo() ?? { name: "campaigns" });
  } catch (err) {
    // Drop the token only when the server rejected it; a transient failure keeps the session.
    if (ConnectError.from(err).code === Code.Unauthenticated) auth.logout();
    error.value = "Strava connected, but we couldn't load your profile. Try connecting again.";
  }
});
</script>

<template>
  <AuthShell>
    <!-- Mounted from the start so the swap from "signing in" to an error is announced. -->
    <div aria-live="polite">
      <CampaignBib v-if="error" blue :band="['No. —', 'Unclaimed']">
        <div class="claim">
          <h2>Not signed in</h2>
          <p>{{ error }}</p>
          <router-link to="/login" class="pill pill--primary">Back to sign in</router-link>
        </div>
      </CampaignBib>
      <CampaignBib v-else blue :band="['No. —', 'One moment']">
        <div class="claim">
          <h2>Signing you in…</h2>
        </div>
      </CampaignBib>
    </div>
  </AuthShell>
</template>

<style scoped>
/* The default bib body bottom-aligns a numeral; here it holds a stacked claim. */
.claim {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-3);
  padding-block: var(--space-3) var(--space-2);
}
h2 {
  margin: 0;
  overflow-wrap: anywhere;
  font: 400 clamp(2rem, 10vw, 3.25rem) / 1 var(--font-display);
  text-transform: uppercase;
  text-wrap: balance;
}
.claim p {
  margin: 0 0 var(--space-3);
  max-width: 32ch;
  overflow-wrap: anywhere;
  font-size: 1.0625rem;
  line-height: 1.45;
  text-wrap: balance;
}
.pill {
  min-height: 2.75rem;
  max-width: 100%;
  text-align: center;
  text-decoration: none;
}
</style>
