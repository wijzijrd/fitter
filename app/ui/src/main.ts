import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import router from "./router";
import "@fontsource/bungee/400.css";
import "@fontsource-variable/hanken-grotesk/wght.css";
import "./style.css";
import { vBusy } from "./lib/busy";
import { vFocus } from "./lib/focus";

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.directive("busy", vBusy);
app.directive("focus", vFocus);
app.mount("#app");
