// Checks the token file against the rules BRAND.md states: contrast in both
// themes, every event type the API sends, and no two types that some reader
// cannot tell apart by color drawn in the same shape.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { contrast, distance, VISION } from "./color.mjs";
import { css, load } from "./css.mjs";

const tokens = load();
const THEMES = ["light", "dark"];
const SHAPES = ["circle", "square", "diamond", "triangle", "triangle-down", "hexagon"];
// Closer than this in OKLab (x100), under normal sight or a simulated deficiency, two marks read as one color.
const CONFUSABLE = 8;

const schema = JSON.parse(readFileSync(new URL("../docs/api/v2/schema.json", import.meta.url), "utf8"));
const apiTypes = schema.$defs.Event.properties.type.enum;
const apiLevels = schema.$defs.Severity.properties.level.enum;

test("both themes define the same colors, all #rrggbb", () => {
  assert.deepEqual(Object.keys(tokens.color.dark), Object.keys(tokens.color.light));
  for (const theme of THEMES) for (const v of Object.values(tokens.color[theme])) assert.match(v, /^#[0-9a-f]{6}$/);
  for (const e of Object.values(tokens.event)) for (const theme of THEMES) assert.match(e[theme], /^#[0-9a-f]{6}$/);
});

test("every event type the API sends has a label, a shape and both colors", () => {
  assert.deepEqual(Object.keys(tokens.event).sort(), [...apiTypes].sort());
  for (const [type, e] of Object.entries(tokens.event)) {
    assert.ok(e.label, type);
    assert.ok(SHAPES.includes(e.shape), `${type}: unknown shape ${e.shape}`);
    // The circle is the quake's magnitude ring; another circle would be lost among thousands of them.
    assert.equal(e.shape === "circle", type === "earthquake", `${type}: only earthquakes are circles`);
  }
});

test("every severity level the API sends, plus ungraded, has a word and its own fill", () => {
  assert.deepEqual(Object.keys(tokens.severity).sort(), [...apiLevels, "ungraded"].sort());
  const fills = Object.values(tokens.severity).map((s) => s.fill);
  assert.equal(new Set(fills).size, fills.length);
});

// [foreground, background, minimum]: text 4.5:1, marks and functional rules 3:1.
// The coastline, graticule and hairline rules are decorative and not listed.
const PAIRS = [
  ["ink", "paper", 4.5], ["ink", "sheet", 4.5], ["ink", "land", 4.5], ["ink", "ocean", 4.5],
  ["muted", "paper", 4.5], ["muted", "sheet", 4.5],
  ["action", "paper", 4.5], ["action", "sheet", 4.5], ["on-action", "action", 4.5],
  ["ink", "warn-bg", 4.5], ["warn", "warn-bg", 3],
];

for (const theme of THEMES) {
  test(`${theme}: text and rules reach their contrast`, () => {
    const c = tokens.color[theme];
    for (const [fg, bg, min] of PAIRS) {
      const r = contrast(c[fg], c[bg]);
      assert.ok(r >= min, `${fg} on ${bg}: ${r.toFixed(2)}:1, needs ${min}:1`);
    }
  });

  test(`${theme}: every event mark reaches 3:1 on the sheet, the land and the ocean`, () => {
    const c = tokens.color[theme];
    for (const [type, e] of Object.entries(tokens.event)) {
      for (const ground of ["sheet", "land", "ocean"]) {
        const r = contrast(e[theme], c[ground]);
        assert.ok(r >= 3, `${type} on ${ground}: ${r.toFixed(2)}:1`);
      }
    }
  });

  test(`${theme}: types whose colors some reader confuses differ in shape`, () => {
    const types = Object.entries(tokens.event);
    for (let i = 0; i < types.length; i++) {
      for (let j = i + 1; j < types.length; j++) {
        const [[a, ea], [b, eb]] = [types[i], types[j]];
        if (ea.shape !== eb.shape) continue;
        for (const vision of VISION) {
          const d = distance(ea[theme], eb[theme], vision);
          assert.ok(d >= CONFUSABLE, `${a} and ${b} are both ${ea.shape}s and ${d.toFixed(1)} apart under ${vision}`);
        }
      }
    }
  });
}

test("the stylesheet carries every color in both themes and the dark block twice", () => {
  const out = css(tokens);
  for (const theme of THEMES) {
    for (const [k, v] of Object.entries(tokens.color[theme])) assert.ok(out.includes(`--${k}: ${v};`), `${theme} ${k}`);
    for (const [k, e] of Object.entries(tokens.event)) assert.ok(out.includes(`--event-${k.replaceAll("_", "-")}: ${e[theme]};`), `${theme} ${k}`);
  }
  assert.equal(out.split("color-scheme: dark;").length - 1, 2);
  assert.ok(out.includes(':root:not([data-theme="light"])'));
});

test("the logo files draw in the tokens' colors", () => {
  const svg = (f) => readFileSync(new URL(`./logo/${f}`, import.meta.url), "utf8");
  const { light, dark } = tokens.color;
  const mark = svg("mark.svg");
  for (const c of [light.ink, light.action, dark.ink, dark.action]) assert.ok(mark.includes(c), `mark.svg lacks ${c}`);
  const icon = svg("icon.svg");
  for (const c of [light.sheet, light.ink, light.action]) assert.ok(icon.includes(c), `icon.svg lacks ${c}`);
  for (const f of ["mark.svg", "icon.svg"]) {
    const used = svg(f).match(/#[0-9a-f]{6}/g);
    const known = new Set([...Object.values(light), ...Object.values(dark)]);
    for (const c of used) assert.ok(known.has(c), `${f} uses ${c}, which is not a token`);
  }
});

test("BRAND.md shows exactly the token file's colors", () => {
  const doc = readFileSync(new URL("./BRAND.md", import.meta.url), "utf8");
  const inDoc = new Set(doc.match(/#[0-9a-f]{6}/g));
  const inTokens = new Set([
    ...THEMES.flatMap((t) => Object.values(tokens.color[t])),
    ...Object.values(tokens.event).flatMap((e) => THEMES.map((t) => e[t])),
  ]);
  assert.deepEqual([...inDoc].filter((c) => !inTokens.has(c)), [], "in BRAND.md but not a token");
  assert.deepEqual([...inTokens].filter((c) => !inDoc.has(c)), [], "a token BRAND.md does not show");
});

test("color math matches known values", () => {
  assert.equal(contrast("#000000", "#ffffff").toFixed(2), "21.00");
  assert.equal(contrast("#777777", "#ffffff").toFixed(2), "4.48");
  assert.equal(distance("#ff0000", "#ff0000", "deuteranopia"), 0);
  // Red and green are far apart to normal sight and close under deuteranopia.
  assert.ok(distance("#d62728", "#2ca02c") > 25);
  assert.ok(distance("#d62728", "#2ca02c", "deuteranopia") < distance("#d62728", "#2ca02c") / 2);
});
