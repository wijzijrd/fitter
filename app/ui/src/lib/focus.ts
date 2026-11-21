import { nextTick, type Directive } from "vue";

// v-focus="cond" focuses the element when it mounts with a truthy value. Used where a
// control swaps for another (Delete -> Keep / Delete), so keyboard focus follows the swap
// instead of dropping to <body>.
export const vFocus: Directive<HTMLElement, boolean> = {
  mounted(el, b) {
    if (b.value) el.focus();
  },
};

// After a row is removed, keep keyboard focus in the list: the control now in the removed
// row's slot, else the one before it, else the fallback (a heading with tabindex="-1").
export function focusNthOr(selector: string, i: number, fallback: string): void {
  nextTick(() => {
    const els = document.querySelectorAll<HTMLElement>(selector);
    (els[i] ?? els[i - 1] ?? document.querySelector<HTMLElement>(fallback))?.focus();
  });
}
