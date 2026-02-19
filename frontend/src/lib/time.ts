export const TIME_PRESETS = [
  { label: "24h", value: "24h" },
  { label: "7d", value: "7d" },
  { label: "30d", value: "30d" },
] as const;

export type TimePreset = (typeof TIME_PRESETS)[number]["value"];

export function getSinceDate(preset: TimePreset): string {
  const now = new Date();
  switch (preset) {
    case "24h":
      return new Date(now.getTime() - 24 * 60 * 60 * 1000).toISOString();
    case "7d":
      return new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000).toISOString();
    case "30d":
      return new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000).toISOString();
  }
}

export function getActivePreset(since?: string): TimePreset | null {
  if (!since) return null;
  const sinceMs = new Date(since).getTime();
  const nowMs = Date.now();
  const diff = nowMs - sinceMs;
  const hour = 60 * 60 * 1000;
  if (Math.abs(diff - 24 * hour) < hour) return "24h";
  if (Math.abs(diff - 7 * 24 * hour) < hour) return "7d";
  if (Math.abs(diff - 30 * 24 * hour) < 2 * hour) return "30d";
  return null;
}
