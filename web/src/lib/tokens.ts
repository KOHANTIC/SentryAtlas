import tokens from "../../../brand/tokens.json";

export type Theme = "light" | "dark";
export type EventType = keyof typeof tokens.event;

/** The color and shape every map mark and legend entry for a type uses (brand/BRAND.md § Events). */
export function eventStyle(type: string, theme: Theme): { color: string; shape: string; label: string } {
  const e = (tokens.event as Record<string, { label: string; shape: string; light: string; dark: string }>)[type] ?? tokens.event.other;
  return { color: e[theme], shape: e.shape, label: e.label };
}

/** The map's ground colors for a theme. */
export function mapGround(theme: Theme) {
  const c = tokens.color[theme];
  return { ocean: c.ocean, land: c.land, coast: c.coast, grid: c.grid };
}
