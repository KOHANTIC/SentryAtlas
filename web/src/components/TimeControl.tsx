"use client";

import { useId } from "react";
import { PRESETS, type Preset } from "@/lib/filters";

const PRESET_LABELS: Record<Preset, string> = { "24h": "24 hours", "7d": "7 days", "30d": "30 days" };

/** "36 hours", "1 hour", "2.5 days": how far back the window reaches. */
export function spanLabel(hours: number): string {
  if (hours < 48) return hours === 1 ? "1 hour" : `${hours} hours`;
  const days = Math.round((hours / 24) * 10) / 10;
  return `${days} days`;
}

type Props = {
  time: Preset;
  hours?: number;
  onChange: (time: Preset, hours: number | undefined) => void;
};

/**
 * The time window: a preset picks which snapshot is loaded, and the slider narrows
 * it in the browser. The slider is the browser's own, so arrow keys, Page Up and
 * Down move it, and a click on its track sets it: no drag is needed.
 */
export default function TimeControl({ time, hours, onChange }: Props) {
  const id = useId();
  const most = PRESETS[time].ms / 3_600_000;
  const step = time === "30d" ? 6 : 1;
  const value = hours ?? most;
  return (
    <div className="time" role="group" aria-labelledby={`${id}-label`}>
      <span id={`${id}-label`} className="label">Window</span>
      <div className="time-presets">
        {(Object.keys(PRESETS) as Preset[]).map((p) => (
          <button key={p} type="button" aria-pressed={p === time} onClick={() => onChange(p, undefined)}>
            {PRESET_LABELS[p]}
          </button>
        ))}
      </div>
      <label className="time-slider">
        <span>Last {spanLabel(value)}</span>
        <input
          type="range"
          min={step}
          max={most}
          step={step}
          value={value}
          aria-valuetext={`Last ${spanLabel(value)}`}
          onChange={(e) => {
            const h = Number(e.target.value);
            onChange(time, h >= most ? undefined : h);
          }}
        />
      </label>
      <p className="muted time-note">NWS shows only alerts in force now. USGS, GDACS and NASA EONET go back 30 days.</p>
    </div>
  );
}
