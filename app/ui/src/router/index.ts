import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "../stores/auth";
import LoginView from "../views/LoginView.vue";
import AuthCallbackView from "../views/AuthCallbackView.vue";
import CampaignsView from "../views/CampaignsView.vue";
import CampaignDetailView from "../views/CampaignDetailView.vue";
import GoalsView from "../views/GoalsView.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", name: "login", component: LoginView },
    { path: "/auth/callback", name: "auth-callback", component: AuthCallbackView },
    { path: "/", name: "campaigns", component: CampaignsView, meta: { requiresAuth: true } },
    {
      path: "/campaigns/:id",
      name: "campaign-detail",
      component: CampaignDetailView,
      meta: { requiresAuth: true },
      props: true,
    },
    { path: "/goals", name: "goals", component: GoalsView, meta: { requiresAuth: true } },
  ],
});

router.beforeEach((to) => {
  const auth = useAuthStore();
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: "login" };
  }
});

export default router;
