import { useId } from "react";
import tokens from "../../../brand/tokens.json";
import { svgPath, type Shape } from "@/lib/marks";

/** One mark in SVG, as the map draws it: the shape, the color, and the severity's fill. */
export default function Mark({ shape, color, fill, size = 20 }: { shape: Shape; color: string; fill: string; size?: number }) {
  const clip = useId();
  const d = svgPath(shape);
  const stroke = { stroke: color, strokeWidth: tokens.mark.stroke, fill: "var(--sheet)" };
  return (
    <svg viewBox="-10 -10 20 20" width={size} height={size} aria-hidden="true" className="mark">
      {fill === "dotted" && <path d={svgPath(shape, 0.8)} {...stroke} fill="none" strokeDasharray={tokens.mark.dash} />}
      {fill === "hollow" && <path d={d} {...stroke} />}
      {fill === "half" && (
        <>
          <clipPath id={clip}><rect x="-10" y="-10" width="10" height="20" /></clipPath>
          <path d={d} {...stroke} />
          <path d={d} fill={color} clipPath={`url(#${clip})`} />
        </>
      )}
      {(fill === "full" || fill === "halo") && <path d={d} {...stroke} fill={color} />}
      {fill === "halo" && <path d={svgPath(shape, 1.6)} stroke={color} strokeWidth={1} fill="none" />}
    </svg>
  );
}

/** The mark for an event type and severity level, in the theme's CSS variables. */
export function eventMark(type: string, level: string | undefined) {
  const e = (tokens.event as Record<string, { shape: string }>)[type] ?? tokens.event.other;
  const fill = level ? tokens.severity[level as keyof typeof tokens.severity].fill : tokens.severity.ungraded.fill;
  return { shape: e.shape as Shape, color: `var(--event-${type.replaceAll("_", "-")})`, fill };
}
