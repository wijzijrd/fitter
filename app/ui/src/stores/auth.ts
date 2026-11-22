import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { userClient } from "../lib/connect";
import { getToken, setToken, clearToken } from "../lib/token";
import type { FeedSettings, User } from "../gen/fitter/v1/user_pb";
import { allDetailsOn } from "../lib/feed";

export const useAuthStore = defineStore("auth", () => {
  const user = ref<User | null>(null);
  // localStorage isn't reactive, so mirror the token in a ref the router guard and shell can track.
  const token = ref(getToken());
  const isAuthenticated = computed(() => !!token.value);

  const isAdmin = computed(() => user.value?.role === "admin");
  const isPending = computed(() => user.value?.status === "pending");

  // What the viewer chose to see under feed rows; everything on until the server says otherwise.
  const feedSettings = computed<FeedSettings>(() => user.value?.feedSettings ?? allDetailsOn);

  async function saveFeedSettings(settings: FeedSettings): Promise<void> {
    const resp = await userClient.updateFeedSettings({ feedSettings: settings });
    if (resp.user) user.value = resp.user;
  }

  async function markWhatsNewSeen(release: string): Promise<void> {
    const resp = await userClient.markWhatsNewSeen({ release });
    if (resp.user) user.value = resp.user;
  }

  async function verifyEmail(code: string): Promise<void> {
    const resp = await userClient.verifyEmail({ code });
    if (resp.user) user.value = resp.user;
  }

  async function fetchCurrentUser(): Promise<void> {
    const resp = await userClient.getCurrentUser({});
    user.value = resp.user ?? null;
  }

  function completeLogin(newToken: string): void {
    setToken(newToken);
    token.value = newToken;
  }

  function logout(): void {
    clearToken();
    token.value = null;
    user.value = null;
  }

  return { user, feedSettings, saveFeedSettings, markWhatsNewSeen, verifyEmail, isAdmin, isPending, isAuthenticated, fetchCurrentUser, completeLogin, logout };
});
