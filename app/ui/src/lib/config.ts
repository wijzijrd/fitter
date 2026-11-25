// Fail at startup rather than render a login button that points at "undefined/...".
export const apiBaseUrl = import.meta.env.VITE_API_BASE_URL as string;
if (!apiBaseUrl) {
  throw new Error("VITE_API_BASE_URL is not set; copy app/ui/.env.example to app/ui/.env");
}

// Developer feedback is off unless the build sets VITE_FEEDBACK_ENABLED=true (deploy-web.yml takes
// it from the FEEDBACK_ENABLED GitHub environment variable). Off while SES is in its sandbox (#52).
export const feedbackEnabled = import.meta.env.VITE_FEEDBACK_ENABLED === "true";
