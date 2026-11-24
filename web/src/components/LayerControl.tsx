"use client";

import { useId, useSyncExternalStore } from "react";
import { LAYERS, satelliteDay, type Layer } from "@/lib/layers";

const NAMES: Record<Layer, string> = { radar: "Radar (US)", satellite: "Satellite" };
const noSubscribe = () => () => {};
const dayLabel = (day: string) => new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", timeZone: "UTC" }).format(new Date(`${day}T00:00:00Z`));

type Props = {
  layers?: readonly Layer[];
  onChange: (layers: Layer[]) => void;
};

/** The imagery under the events: each layer a toggle, with a line on what it shows while it is on. */
export default function LayerControl({ layers = [], onChange }: Props) {
  const id = useId();
  // Read in the browser, never at build time, so the day named is the day the map asks for.
  const day = useSyncExternalStore(noSubscribe, () => satelliteDay(Date.now()), () => "");
  const toggle = (l: Layer) => onChange(LAYERS.filter((x) => (x === l ? !layers.includes(x) : layers.includes(x))));
  return (
    <div className="layers" role="group" aria-labelledby={`${id}-label`}>
      <span id={`${id}-label`} className="label">Layers</span>
      <div className="time-presets">
        {LAYERS.map((l) => (
          <button key={l} type="button" aria-pressed={layers.includes(l)} onClick={() => toggle(l)}>
            {NAMES[l]}
          </button>
        ))}
      </div>
      {layers.includes("radar") && (
        <p className="muted layer-note">
          Radar: NOAA&apos;s latest picture, every few minutes, US states and territories only. Blue is light rain or snow,
          then green, yellow and red; purple and white are the heaviest.
        </p>
      )}
      {layers.includes("satellite") && day && (
        <p className="muted layer-note">Satellite: NASA&apos;s true-colour picture of {dayLabel(day)}, from daylight passes.</p>
      )}
    </div>
  );
}
