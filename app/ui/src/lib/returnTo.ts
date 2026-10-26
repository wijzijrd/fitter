// Remembers where a signed-out visitor was headed (e.g. an invite link to
// /campaigns/:id/join) across the Strava OAuth round trip, which is a full
// page navigation that can reopen in a new tab. localStorage survives that
// where sessionStorage doesn't; the expiry keeps an abandoned attempt from
// redirecting a much later login. Storage can throw in private windows, so
// every access degrades to "no saved path".
const KEY = "fitter_return_to";
const MAX_AGE_MS = 30 * 60 * 1000;

// Returning to the login or callback screen would loop or strand the visitor.
const isAuthPath = (path: string) => /^\/(login|auth)(\/|\?|#|$)/.test(path);

export function saveReturnTo(path: string): void {
  if (isAuthPath(path)) return;
  try {
    localStorage.setItem(KEY, JSON.stringify({ path, at: Date.now() }));
  } catch {
    // No storage: the visitor lands on the campaigns list after login.
  }
}

// Read-once: returns the saved in-app path and clears it. Expired entries and
// anything that isn't a plain absolute in-app path ("//host", "/\host") or points at an auth screen are dropped.
export function takeReturnTo(): string | null {
  try {
    const raw = localStorage.getItem(KEY);
    localStorage.removeItem(KEY);
    if (!raw) return null;
    const { path, at } = JSON.parse(raw) as { path?: unknown; at?: unknown };
    if (typeof path !== "string" || typeof at !== "number" || Date.now() - at > MAX_AGE_MS) return null;
    return /^\/(?![/\\])/.test(path) && !isAuthPath(path) ? path : null;
  } catch {
    return null;
  }
}
