import type { Directive } from "vue";

// v-busy marks a button as unavailable with aria-disabled instead of
// `disabled`, so a keyboard user who just pressed it keeps focus (DESIGN.md:
// controls stay focusable; the click is what gets refused). The capture
// listener runs before the button's own @click and form submission.
export function setBusy(el: Pick<HTMLElement, "setAttribute" | "removeAttribute">, on: boolean): void {
  if (on) el.setAttribute("aria-disabled", "true");
  else el.removeAttribute("aria-disabled");
}

export function refuseWhenBusy(el: Pick<HTMLElement, "getAttribute">, ev: Pick<Event, "preventDefault" | "stopImmediatePropagation">): void {
  if (el.getAttribute("aria-disabled") !== "true") return;
  ev.preventDefault();
  ev.stopImmediatePropagation();
}

export const vBusy: Directive<HTMLElement, boolean> = {
  mounted(el, b) {
    setBusy(el, !!b.value);
    el.addEventListener("click", (ev) => refuseWhenBusy(el, ev), true);
  },
  updated(el, b) {
    setBusy(el, !!b.value);
  },
};
