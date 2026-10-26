import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "../stores/auth";
import LoginView from "../views/LoginView.vue";
import AuthCallbackView from "../views/AuthCallbackView.vue";
import { saveReturnTo } from "../lib/returnTo";

// Login and callback stay in the entry chunk; every signed-in view (and ECharts)
// loads on demand so a first-time visitor only downloads the sign-in screen.
const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", name: "login", component: LoginView, meta: { title: "Sign in" } },
    { path: "/auth/callback", name: "auth-callback", component: AuthCallbackView, meta: { title: "Signing in" } },
    { path: "/", name: "campaigns", component: () => import("../views/CampaignsView.vue"), meta: { requiresAuth: true, title: "Campaigns" } },
    { path: "/campaigns/new", name: "campaign-new", component: () => import("../views/CampaignCreateView.vue"), meta: { requiresAuth: true, title: "New campaign" } },
    {
      path: "/campaigns/:id/join",
      name: "campaign-join",
      component: () => import("../views/CampaignJoinView.vue"),
      meta: { requiresAuth: true, title: "Join campaign" },
      props: true,
    },
    {
      path: "/campaigns/:id",
      name: "campaign-detail",
      component: () => import("../views/CampaignDetailView.vue"),
      meta: { requiresAuth: true, title: "Campaign" },
      props: true,
    },
    { path: "/goals", name: "goals", component: () => import("../views/GoalsView.vue"), meta: { requiresAuth: true, title: "Goals" } },
  ],
});

router.beforeEach((to) => {
  const auth = useAuthStore();
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    saveReturnTo(to.fullPath);
    return { name: "login" };
  }
  if (to.name === "login" && auth.isAuthenticated) {
    return { name: "campaigns" };
  }
});

// A deploy swaps the hashed chunks, so a tab opened before it fails to import
// the next route. Reload once into the new build; the flag stops a reload loop
// and clears after the next successful navigation. Offline, staying put is kinder.
const RELOADED = "fitter_chunk_reload";
router.onError((err, to) => {
  if (!/dynamically imported module|importing a module script/i.test(String(err?.message)) || !navigator.onLine) return;
  try {
    if (sessionStorage.getItem(RELOADED)) return;
    sessionStorage.setItem(RELOADED, "1");
  } catch {
    return;
  }
  window.location.assign(to.fullPath);
});

router.afterEach((to) => {
  try {
    sessionStorage.removeItem(RELOADED);
  } catch {
    // No storage: the one-reload guard just can't reset.
  }
  document.title = typeof to.meta.title === "string" ? `${to.meta.title} · Fitter` : "Fitter";
});

export default router;
