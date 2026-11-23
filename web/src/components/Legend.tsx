import tokens from "../../../brand/tokens.json";
import type { Shape } from "@/lib/marks";
import Mark from "./Mark";

const LEVELS = ["extreme", "severe", "moderate", "minor", "ungraded"] as const;

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
