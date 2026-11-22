import { defineStore } from "pinia";
import { ref } from "vue";
import { activityClient } from "../lib/connect";

// Matches the server's own limit (syncMinInterval); asking sooner only returns "throttled".
const MIN_GAP_MS = 5 * 60 * 1000;

export const useSyncStore = defineStore("sync", () => {
  // Activities pulled in during this page load. Views watch it to reload what they show.
  const imported = ref(0);
  let last = 0;
  let running = false;

  // Fire and forget: a failed or throttled sync leaves the data as it was, and the next trigger retries.
  async function sync(): Promise<void> {
    if (running || Date.now() - last < MIN_GAP_MS) return;
    running = true;
    last = Date.now();
    try {
      const resp = await activityClient.syncActivities({});
      if (resp.imported > 0) imported.value += resp.imported;
    } catch {
      last = 0; // let the next trigger try again rather than wait out the gap
    } finally {
      running = false;
    }
  }

  return { imported, sync };
});
