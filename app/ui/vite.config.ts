import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// Picks the icon set in public/icons/<env>/ (index.html). deploy-web.yml passes the environment
// (staging or prod); local dev and tests use the staging set.
process.env.VITE_APP_ENV ||= "staging";

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
  },
});
