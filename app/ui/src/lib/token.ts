// Single source of truth for the app's JWT, shared by the Connect auth
// interceptor and the auth store so neither can drift out of sync.
const STORAGE_KEY = "fitter_token";

export function getToken(): string | null {
  return localStorage.getItem(STORAGE_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(STORAGE_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(STORAGE_KEY);
}
