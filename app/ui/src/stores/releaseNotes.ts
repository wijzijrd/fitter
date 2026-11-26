import { defineStore } from "pinia";
import { ref } from "vue";
import { useAuthStore } from "./auth";
import { CURRENT, shouldPopUp, unseenReleases, type Release } from "../lib/releases";

export const useReleaseNotesStore = defineStore("releaseNotes", () => {
  const auth = useAuthStore();
  const open = ref(false);
  const shown = ref<Release[]>([]);
  // Opened automatically at most once per page load, so closing it isn't undone by the next user refresh.
  let autoOffered = false;

  // Called when the signed-in user is known; pops up when a major or minor release is unseen.
  function offer(): void {
    const u = auth.user;
    if (autoOffered || !u || auth.isPending || !shouldPopUp(u.whatsNewSeen)) return;
    autoOffered = true;
    shown.value = unseenReleases(u.whatsNewSeen);
    open.value = true;
  }

  // A failed save only means it shows again next time, which beats an error over a closed dialog.
  async function dismiss(): Promise<void> {
    open.value = false;
    if (auth.user?.whatsNewSeen !== CURRENT) await auth.markWhatsNewSeen(CURRENT).catch(() => {});
  }

  return { open, shown, offer, dismiss };
});
