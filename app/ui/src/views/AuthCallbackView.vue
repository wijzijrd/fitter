<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";

const router = useRouter();
const auth = useAuthStore();
const error = ref("");

onMounted(async () => {
  const hash = new URLSearchParams(window.location.hash.replace(/^#/, ""));
  const token = hash.get("token");
  if (!token) {
    error.value = "Missing token from login redirect.";
    return;
  }
  auth.completeLogin(token);
  try {
    await auth.fetchCurrentUser();
    router.replace({ name: "campaigns" });
  } catch {
    error.value = "Could not load your profile after login.";
  }
});
</script>

<template>
  <div class="mt-16 text-center">
    <p v-if="error" class="text-sm text-muted-foreground">{{ error }}</p>
    <p v-else class="text-sm text-muted-foreground">Signing you in…</p>
  </div>
</template>
