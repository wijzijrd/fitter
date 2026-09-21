<script setup lang="ts">
import { Button } from "@/components/ui/button";

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL as string;
const loginUrl = `${apiBaseUrl}/auth/strava/login`;
</script>

<template>
  <div class="mx-auto mt-16 max-w-sm px-6 text-center">
    <h1 class="text-2xl font-semibold">Fitter</h1>
    <p class="mt-2 text-sm text-muted-foreground">
      Track activities and campaigns with your friends.
    </p>
    <Button as="a" :href="loginUrl" class="mt-6">Log in with Strava</Button>
  </div>
</template>
