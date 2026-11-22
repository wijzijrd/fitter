import { defineStore } from "pinia";
import { ref } from "vue";
import { useAuthStore } from "./auth";
import { RELEASE, isFresh } from "../lib/whatsNew";

export const useWhatsNewStore = defineStore("whatsNew", () => {
  const auth = useAuthStore();
  const open = ref(false);
  // Opened automatically at most once per page load, so closing it isn't undone by the next user refresh.
  let autoOffered = false;

  // Called when the signed-in user is known; shows the splash if this release is fresh and hasn't been dismissed.
  function offer(): void {
    const u = auth.user;
    if (autoOffered || !u || auth.isPending || u.whatsNewSeen === RELEASE || !isFresh()) return;
    autoOffered = true;
    open.value = true;
  }

  function show(): void {
    open.value = true;
  }

  // A failed save only means it shows again next time, which beats an error over a dismissed dialog.
  async function dismiss(): Promise<void> {
    open.value = false;
    if (auth.user?.whatsNewSeen !== RELEASE) await auth.markWhatsNewSeen(RELEASE).catch(() => {});
  }

  return { open, offer, show, dismiss };
});
