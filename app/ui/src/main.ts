import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import router from "./router";
import { THEME_STORAGE_KEY } from "./lib/theme";
import "./index.css";

// Apply the dark/light theme class BEFORE mounting so there's no flash of
// the wrong theme: prefer an explicit stored choice, falling back to the
// OS-level preference when nothing has been chosen yet.
function bootstrapTheme(): void {
  let stored: string | null = null;
  try {
    stored = localStorage.getItem(THEME_STORAGE_KEY);
  } catch {
    // localStorage can throw in some environments (private browsing, etc.)
    // — fall back to the OS preference below.
  }
  const dark = stored ? stored === "dark" : window.matchMedia("(prefers-color-scheme: dark)").matches;
  document.documentElement.classList.toggle("dark", dark);
}
bootstrapTheme();

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.mount("#app");
