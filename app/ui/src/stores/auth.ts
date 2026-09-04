import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { userClient } from "../lib/connect";
import { getToken, setToken, clearToken } from "../lib/token";
import type { User } from "../gen/fitter/v1/user_pb";

export const useAuthStore = defineStore("auth", () => {
  const user = ref<User | null>(null);
  const isAuthenticated = computed(() => !!getToken());

  async function fetchCurrentUser(): Promise<void> {
    const resp = await userClient.getCurrentUser({});
    user.value = resp.user ?? null;
  }

  function completeLogin(token: string): void {
    setToken(token);
  }

  function logout(): void {
    clearToken();
    user.value = null;
  }

  return { user, isAuthenticated, fetchCurrentUser, completeLogin, logout };
});
