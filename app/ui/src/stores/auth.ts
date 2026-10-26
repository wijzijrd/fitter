import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { userClient } from "../lib/connect";
import { getToken, setToken, clearToken } from "../lib/token";
import type { User } from "../gen/fitter/v1/user_pb";

export const useAuthStore = defineStore("auth", () => {
  const user = ref<User | null>(null);
  // localStorage isn't reactive, so mirror the token in a ref the router guard and shell can track.
  const token = ref(getToken());
  const isAuthenticated = computed(() => !!token.value);

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

  return { user, isAuthenticated, fetchCurrentUser, completeLogin, logout };
});
