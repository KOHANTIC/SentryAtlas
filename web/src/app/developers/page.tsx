import type { Metadata } from "next";
import { TextPage } from "@/components/Site";
import { API_BASE } from "@/lib/snapshot";

export const metadata: Metadata = {
  title: "Developers",
  description: "The SentryAtlas API: every current event from four public sources, free and without a key.",
  alternates: { canonical: "https://sentryatlas.com/developers/" },
};

const DOCS = "https://github.com/KOHANTIC/SentryAtlas/blob/main";

/** The API, in short, with links to the full reference. */
export default function Developers() {
  return (
    <TextPage label="Developers" title="The API">
      <p className="lede">
        The events on the map, as JSON, free and without a key. Both versions are open to any site, and allow 60 requests
        a minute from one address.
      </p>

      <h2>Version 2</h2>
      <table>
        <thead>
          <tr><th>Request</th><th>Answer</th></tr>
        </thead>
        <tbody>
          <tr><td><code>GET /api/v2/snapshot?window=7d</code></td><td>Every current event updated in the last 7 days (or <code>30d</code>), with each source&apos;s status. Cached for 30 seconds; send <code>If-None-Match</code> to get <code>304</code> when nothing changed.</td></tr>
          <tr><td><code>GET /api/v2/events/&#123;id&#125;</code></td><td>One event in full, with the source&apos;s own text. <code>404</code> once it has left its source&apos;s feed.</td></tr>
        </tbody>
      </table>
      <pre tabIndex={0} aria-label="Example: the 7-day snapshot"><code>{`curl ${API_BASE}/api/v2/snapshot?window=7d`}</code></pre>
      <p>
        The model, with every field and what it means: <a href={`${DOCS}/docs/api/v2/README.md`}>docs/api/v2</a>, and its{" "}
        <a href={`${DOCS}/docs/api/v2/schema.json`}>JSON Schema</a>.
      </p>

      <h2>Version 1</h2>
      <p>
        <code>GET /api/v1/events</code> filters by type, place and time, and answers GeoJSON, flat JSON, or a stream of
        server-sent events. It stays as it is: <a href={`${DOCS}/backend/README.md`}>its reference</a>.
      </p>
      <pre tabIndex={0} aria-label="Example: earthquakes since 20 November, version 1"><code>{`curl "${API_BASE}/api/v1/events?types=earthquake&since=2026-11-20&format=json"`}</code></pre>

      <h2>Health</h2>
      <p>
        <code>GET /health</code> says how fresh each source is. <code>GET /health/stale</code> answers <code>503</code> while
        any source is past its limit, for a monitor.
      </p>

      <h2>Using the data</h2>
      <p>
        Credit the agencies the events come from: USGS, NASA EONET, the National Weather Service and GDACS. SentryAtlas
        is not an official warning service. The code is open source: <a href="https://github.com/KOHANTIC/SentryAtlas">KOHANTIC/SentryAtlas</a>.
      </p>
    </TextPage>
  );
}
