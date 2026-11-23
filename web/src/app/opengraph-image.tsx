import { ImageResponse } from "next/og";
import tokens from "../../../brand/tokens.json";

export const dynamic = "force-static";
export const alt = "SentryAtlas: what the world's hazard services are reporting, on one map.";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

const c = tokens.color.light;

/** The share card: the sheet mark, the name and the line, on the light sheet. */
export default function OpenGraphImage() {
  return new ImageResponse(
    (
      <div style={{ width: "100%", height: "100%", display: "flex", flexDirection: "column", justifyContent: "center", gap: 40, padding: 96, background: c.sheet, color: c.ink, border: `16px solid ${c.paper}` }}>
        <div style={{ display: "flex", alignItems: "center", gap: 28 }}>
          <svg viewBox="0 0 26 26" width="120" height="120">
            <rect x="1" y="1" width="24" height="24" fill="none" stroke={c.ink} strokeWidth="2" />
            <path d="M4 17c4-5 7-1 10-5s5-4 8-2" fill="none" stroke={c.action} strokeWidth="2" />
            <circle cx="17" cy="9" r="2.4" fill={c.ink} />
          </svg>
          <div style={{ fontSize: 96, fontWeight: 800, letterSpacing: 6 }}>SENTRYATLAS</div>
        </div>
        <div style={{ fontSize: 48, lineHeight: 1.25, maxWidth: 960 }}>What the world&apos;s hazard services are reporting, on one map.</div>
      </div>
    ),
    size,
  );
}
