<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import AuthShell from "../components/AuthShell.vue";
import CampaignBib from "../components/CampaignBib.vue";
import { apiBaseUrl } from "../lib/config";
import connectWithStrava from "../assets/strava/btn_strava_connect_with_orange.svg";

const loginUrl = `${apiBaseUrl}/auth/strava/login`;

// The backend redirects here with ?error=<code>: access_denied when the
// visitor cancels on Strava, or one of the codes named in handler/auth.go
// (errState, errScope, errExchange, errServer). The raw value is never rendered.
const messages: Record<string, string> = {
  access_denied: "You cancelled on Strava, so nothing was connected. Try again when you're ready.",
  state: "Sign-in timed out or your browser blocked a cookie. Try connecting again.",
  scope: "Fitter needs to see your activities. Connect again and keep 'View data about your activities' ticked.",
};
const route = useRoute();
const error = computed(() => {
  const e = route.query.error;
  if (!e) return "";
  return (typeof e === "string" && messages[e]) || "Sign-in didn't finish. Try again.";
});
</script>

<template>
  <AuthShell>
    <!-- Region stays mounted so the notice is announced when it appears. -->
    <div role="status">
      <p v-if="error" class="notice">{{ error }}</p>
    </div>
    <CampaignBib blue :band="['No. —', 'Unclaimed']" tab="Your activities, private ones too, show in the feed of campaigns you join. Goals stay yours.">
      <div class="claim">
        <h2>Get your bib</h2>
        <p>Your Strava activities, your crew's challenges.</p>
        <!-- Strava's official button, unmodified, per their brand guidelines. -->
        <a class="strava-connect" :href="loginUrl">
          <img :src="connectWithStrava" alt="Connect with Strava" width="237" height="48" />
        </a>
      </div>
    </CampaignBib>
  </AuthShell>
</template>

<style scoped>
.notice {
  margin: 0 0 1rem;
  padding: 0.75rem 1rem;
  border: 2px solid var(--ink);
  border-radius: 10px;
  background: var(--cool-blue);
  font-weight: 700;
}
/* The default bib body bottom-aligns a numeral; here it holds a stacked claim. */
.claim {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.6rem;
  padding-block: 0.5rem 0.4rem;
}
h2 {
  margin: 0;
  overflow-wrap: anywhere;
  font: 400 clamp(2rem, 10vw, 3.25rem) / 1 var(--font-display);
  text-transform: uppercase;
  text-wrap: balance;
}
.claim p {
  margin: 0 0 0.6rem;
  max-width: 28ch;
  overflow-wrap: anywhere;
  font-size: 1.0625rem;
  line-height: 1.45;
  text-wrap: balance;
}
.strava-connect {
  display: inline-block;
  border-radius: 6px;
}
.strava-connect img {
  display: block;
  max-width: 100%;
  height: auto;
}
.strava-connect:focus-visible {
  outline: 3px solid var(--ink);
  outline-offset: 3px;
}
</style>
