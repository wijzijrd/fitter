<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import AuthShell from "../components/AuthShell.vue";
import CampaignBib from "../components/CampaignBib.vue";

const auth = useAuthStore();
const router = useRouter();
const checking = ref(false);
const error = ref("");

async function recheck(): Promise<void> {
  checking.value = true;
  error.value = "";
  try {
    await auth.fetchCurrentUser();
    if (!auth.isPending) router.replace({ name: "campaigns" });
  } catch {
    error.value = "Couldn't check right now. Try again.";
  } finally {
    checking.value = false;
  }
}

function logout(): void {
  auth.logout();
  router.push({ name: "login" });
}
</script>

<template>
  <AuthShell>
    <CampaignBib blue :band="['No. —', 'Pending']">
      <div class="claim" aria-live="polite">
        <h2>Waiting for approval</h2>
        <p>An admin needs to approve your account before you can race. Check back soon.</p>
        <p v-if="error" role="alert">{{ error }}</p>
        <button type="button" class="pill pill--primary" :disabled="checking" @click="recheck">Check again</button>
        <button type="button" class="pill" @click="logout">Log out</button>
      </div>
    </CampaignBib>
  </AuthShell>
</template>

<style scoped>
.claim {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-3);
  padding-block: var(--space-3) var(--space-2);
}
h2 {
  margin: 0;
  font: 400 clamp(2rem, 10vw, 3.25rem) / 1 var(--font-display);
  text-transform: uppercase;
  text-wrap: balance;
}
.claim p {
  margin: 0 0 var(--space-3);
  max-width: 32ch;
  font-size: 1.0625rem;
  line-height: 1.45;
}
.pill {
  min-height: 2.75rem;
}
</style>
