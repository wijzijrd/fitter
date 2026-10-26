// Fail at startup rather than render a login button that points at "undefined/...".
export const apiBaseUrl = import.meta.env.VITE_API_BASE_URL as string;
if (!apiBaseUrl) {
  throw new Error("VITE_API_BASE_URL is not set; copy app/ui/.env.example to app/ui/.env");
}
