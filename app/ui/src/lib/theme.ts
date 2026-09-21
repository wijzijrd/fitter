// resolveThemeColor reads a CSS custom property (e.g. "--primary") and
// resolves it to a concrete rgb()/rgba() string ECharts can use as a
// literal color, following the active theme (light/dark) at render time.
//
// getComputedStyle().getPropertyValue() on a custom property returns its
// raw declared value verbatim — for tokens declared as oklch(...) (this
// app's theme, see style.css) that's a string ECharts/zrender can't parse.
// Assigning the property through `color: var(--x)` on a probe element and
// reading the browser's *computed* `color` back forces the browser to
// resolve oklch/color-mix/etc. into a plain rgb()/rgba() string, which is
// how this function gets a literal ECharts can use.
export function resolveThemeColor(varName: string, fallback = "#888888"): string {
  if (typeof document === "undefined") return fallback;
  const probe = document.createElement("span");
  probe.style.position = "absolute";
  probe.style.visibility = "hidden";
  probe.style.color = `var(${varName})`;
  document.body.appendChild(probe);
  const resolved = getComputedStyle(probe).color;
  document.body.removeChild(probe);
  return resolved || fallback;
}
