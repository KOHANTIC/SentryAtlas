import tokens from "../../../brand/tokens.json";
import { svgPath, type Shape } from "@/lib/marks";

const LEVELS = ["extreme", "severe", "moderate", "minor", "ungraded"] as const;

/** One mark in SVG, in the type's color from the theme's CSS variables. */
function Mark({ shape, color, fill }: { shape: Shape; color: string; fill: string }) {
  const d = svgPath(shape);
  const stroke = { stroke: color, strokeWidth: tokens.mark.stroke, fill: "var(--sheet)" };
  return (
    <svg viewBox="-10 -10 20 20" width="20" height="20" aria-hidden="true">
      {fill === "dotted" && <path d={svgPath(shape, 0.8)} {...stroke} fill="none" strokeDasharray={tokens.mark.dash} />}
      {fill === "hollow" && <path d={d} {...stroke} />}
      {fill === "half" && (
        <>
          <clipPath id={`half-${shape}`}><rect x="-10" y="-10" width="10" height="20" /></clipPath>
          <path d={d} {...stroke} />
          <path d={d} fill={color} clipPath={`url(#half-${shape})`} />
        </>
      )}
      {(fill === "full" || fill === "halo") && <path d={d} {...stroke} fill={color} />}
      {fill === "halo" && <path d={svgPath(shape, 1.6)} stroke={color} strokeWidth={1} fill="none" />}
    </svg>
  );
}

/** What the map draws: one mark per type, the severity fills, and the quake rings. */
export default function Legend() {
  return (
    <section className="legend" aria-labelledby="legend-title">
      <h2 id="legend-title" className="label">Key</h2>
      <ul className="legend-types">
        {Object.entries(tokens.event).map(([type, e]) => (
          <li key={type}>
            <Mark shape={e.shape as Shape} color={`var(--event-${type.replaceAll("_", "-")})`} fill="full" />
            {e.label}
          </li>
        ))}
      </ul>
      <h3 className="label">Severity</h3>
      <ul className="legend-levels">
        {LEVELS.map((l) => (
          <li key={l}>
            <Mark shape="diamond" color="var(--ink)" fill={tokens.severity[l].fill} />
            {tokens.severity[l].word}
          </li>
        ))}
      </ul>
      <p className="legend-note">Earthquakes are rings sized by magnitude. A numbered circle holds that many earthquakes.</p>
    </section>
  );
}
