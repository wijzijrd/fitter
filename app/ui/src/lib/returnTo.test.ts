import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { saveReturnTo, takeReturnTo } from "./returnTo";

function stubStorage() {
  const data = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
    removeItem: (k: string) => void data.delete(k),
  });
  return data;
}

describe("returnTo", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    stubStorage();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("returns a saved path once", () => {
    saveReturnTo("/campaigns/abc/join?x=1");
    expect(takeReturnTo()).toBe("/campaigns/abc/join?x=1");
    expect(takeReturnTo()).toBeNull();
  });

  it("drops an entry older than 30 minutes", () => {
    saveReturnTo("/goals");
    vi.advanceTimersByTime(31 * 60 * 1000);
    expect(takeReturnTo()).toBeNull();
  });

  it.each(["//evil.example", "/\\evil.example", "https://evil.example", "goals"])("rejects %s", (path) => {
    saveReturnTo(path);
    expect(takeReturnTo()).toBeNull();
  });

  it.each(["/login", "/login?error=state", "/auth/callback", "/auth/callback#token=x"])("never saves auth path %s", (path) => {
    saveReturnTo(path);
    expect(takeReturnTo()).toBeNull();
  });

  it("does not restore an auth path that was already stored", () => {
    localStorage.setItem("fitter_return_to", JSON.stringify({ path: "/auth/callback", at: Date.now() }));
    expect(takeReturnTo()).toBeNull();
  });

  it("returns null when storage throws", () => {
    vi.stubGlobal("localStorage", {
      getItem: () => {
        throw new Error("denied");
      },
      setItem: () => {
        throw new Error("denied");
      },
      removeItem: () => {},
    });
    expect(() => saveReturnTo("/goals")).not.toThrow();
    expect(takeReturnTo()).toBeNull();
  });
});
