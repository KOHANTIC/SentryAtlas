// Writes the tokens as CSS custom properties: light on :root, dark under the
// device setting unless the page chose light, and dark again when it chose dark.
// Run `node brand/css.mjs` to print them.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

/** load reads brand/tokens.json. */
export function load() {
  return JSON.parse(readFileSync(new URL("./tokens.json", import.meta.url), "utf8"));
}

/** css returns the stylesheet text for a token set. */
export function css(tokens) {
  const colors = (theme) => [
    ...Object.entries(tokens.color[theme]).map(([k, v]) => `--${k}: ${v};`),
    ...Object.entries(tokens.event).map(([k, v]) => `--event-${k.replaceAll("_", "-")}: ${v[theme]};`),
  ];
  const fixed = [
    ...Object.entries(tokens.font).map(([k, v]) => `--font-${k}: "${v.family}", ${v.fallback};`),
    `--label-tracking: ${tokens.font.label.tracking};`,
    `--wordmark-tracking: ${tokens.font.label["wordmark-tracking"]};`,
    ...Object.entries(tokens.size).map(([k, v]) => `--size-${k}: ${v};`),
    ...Object.entries(tokens.space).map(([k, v]) => `--${k}: ${v};`),
    `--focus-width: ${tokens.focus.width};`,
    `--focus-offset: ${tokens.focus.offset};`,
  ];
  const block = (selector, lines, indent = "") =>
    `${indent}${selector} {\n${lines.map((l) => `${indent}  ${l}`).join("\n")}\n${indent}}`;
  const dark = [...colors("dark"), "color-scheme: dark;"];
  return [
    block(":root", [...colors("light"), ...fixed, "color-scheme: light;"]),
    `@media (prefers-color-scheme: dark) {\n${block(':root:not([data-theme="light"])', dark, "  ")}\n}`,
    block(':root[data-theme="dark"]', dark),
    "",
  ].join("\n");
}

if (process.argv[1] === fileURLToPath(import.meta.url)) process.stdout.write(css(load()));
