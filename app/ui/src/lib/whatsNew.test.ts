import { describe, expect, it } from "vitest";
import { isFresh } from "./whatsNew";

describe("isFresh", () => {
  const day = 86_400_000;
  const t = Date.parse("2026-11-22");
  it("offers the release for a week", () => {
    expect(isFresh(t, "2026-11-22")).toBe(true);
    expect(isFresh(t + 6 * day + day - 1, "2026-11-22")).toBe(true);
  });
  it("stops offering it after seven days", () => {
    expect(isFresh(t + 7 * day, "2026-11-22")).toBe(false);
  });
});
