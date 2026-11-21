import { describe, expect, it, vi } from "vitest";
import { refuseWhenBusy, setBusy } from "./busy";

function fakeEl() {
  const attrs = new Map<string, string>();
  return {
    setAttribute: (k: string, v: string) => void attrs.set(k, v),
    removeAttribute: (k: string) => void attrs.delete(k),
    getAttribute: (k: string) => attrs.get(k) ?? null,
  };
}

describe("v-busy", () => {
  it("refuses clicks while busy and lets them through after", () => {
    const el = fakeEl();
    const ev = { preventDefault: vi.fn(), stopImmediatePropagation: vi.fn() };
    setBusy(el, true);
    refuseWhenBusy(el, ev);
    expect(ev.preventDefault).toHaveBeenCalled();
    expect(ev.stopImmediatePropagation).toHaveBeenCalled();

    const ev2 = { preventDefault: vi.fn(), stopImmediatePropagation: vi.fn() };
    setBusy(el, false);
    refuseWhenBusy(el, ev2);
    expect(ev2.preventDefault).not.toHaveBeenCalled();
    expect(el.getAttribute("aria-disabled")).toBeNull();
  });
});
