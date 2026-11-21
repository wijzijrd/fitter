import type { Directive } from "vue";

// v-focus="cond" focuses the element when it mounts with a truthy value. Used where a
// control swaps for another (Delete -> Keep / Delete), so keyboard focus follows the swap
// instead of dropping to <body>.
export const vFocus: Directive<HTMLElement, boolean> = {
  mounted(el, b) {
    if (b.value) el.focus();
  },
};
