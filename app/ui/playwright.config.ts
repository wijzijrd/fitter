import { defineConfig } from "@playwright/test";

// Browser checks for UI behaviour unit tests can't reach: focus movement, touch target sizes,
// layout at phone and tablet widths. The API is mocked per test (tests/e2e/api.ts), so no
// backend is needed. Run with `npm run test:e2e` (first time: `npx playwright install chromium-headless-shell`).
const PORT = 5199;

export default defineConfig({
  testDir: "tests/e2e",
  testMatch: "**/*.e2e.ts",
  use: { baseURL: `http://127.0.0.1:${PORT}` },
  webServer: {
    command: `vite --port ${PORT} --strictPort --host 127.0.0.1`,
    url: `http://127.0.0.1:${PORT}`,
    env: { VITE_API_BASE_URL: `http://127.0.0.1:${PORT}/api` },
    reuseExistingServer: !process.env.CI,
  },
});
