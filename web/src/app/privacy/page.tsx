import type { Metadata } from "next";
import { TextPage } from "@/components/Site";
import { OPERATOR, PRIVACY_EMAIL } from "@/lib/operator";

export const metadata: Metadata = {
  title: "Privacy",
  description: "What SentryAtlas counts, what its servers write down, and which other services your browser contacts.",
  alternates: { canonical: "https://sentryatlas.com/privacy/" },
};

const UPDATED = "23 November 2026";
const HISTORY = "https://github.com/KOHANTIC/SentryAtlas/commits/main/web/src/app/privacy/page.tsx";
const Mail = () => (PRIVACY_EMAIL ? <a href={`mailto:${PRIVACY_EMAIL}`}>{PRIVACY_EMAIL}</a> : null);

/** The privacy notice, as approved on 2026-11-22 and 2026-11-23. */
export default function Privacy() {
  return (
    <TextPage label="Privacy" title="Privacy">
      <p className="lede">
        SentryAtlas has no ads, no tracking cookies and no accounts. This page says what we count, what our servers
        write down, and which other services your browser contacts.
      </p>
      <p className="muted">Last changed {UPDATED}.</p>

      <h2>Who runs it</h2>
      {OPERATOR ? (
        <p>
          SentryAtlas at sentryatlas.com is run by {OPERATOR}.{PRIVACY_EMAIL && <> Questions about privacy go to <Mail />.</>}
        </p>
      ) : (
        <p>This copy of SentryAtlas does not name its operator.</p>
      )}

      <h2>What we count</h2>
      <p>
        We count visits with <a href="https://www.cloudflare.com/web-analytics/">Cloudflare Web Analytics</a>. For each
        page view it records the page, the site that linked to it, your country, browser, operating system and type of
        device, and how fast the page loaded. It does not record the part of the address after a question mark.
      </p>
      <p>
        It sets no cookies and stores nothing in your browser. Cloudflare says it does not fingerprint visitors or follow
        them from site to site. We see totals, not people. Cloudflare keeps full detail for 7 days and a sample after
        that; we can look back six months.
      </p>

      <h2>What our servers write down</h2>
      <p>
        For each request, the API logs the method, the path, the status, the size, how long it took and a random request
        ID. It does not log your IP address, your browser, or the part of the address after a question mark. Each hour
        it also counts calls by format, and by whether they came from our own pages, another site, or neither.
      </p>
      <p>To apply its rate limit, the API keeps your IP address in memory for up to two minutes. It never writes it down.</p>
      <p>
        <a href="https://www.cloudflare.com/privacypolicy/">Cloudflare</a> hosts the site and carries every request, and{" "}
        <a href="https://www.digitalocean.com/legal/privacy-policy">DigitalOcean</a> hosts the API. Like any host they see
        your IP address, and their own policies cover what they keep.
      </p>

      <h2>Who else your browser talks to</h2>
      <ul>
        <li>Cloudflare delivers the site, and the analytics script from static.cloudflareinsights.com.</li>
        <li>
          <a href="https://openfreemap.org/">OpenFreeMap</a> (tiles.openfreemap.org) draws the map underneath the events:
          land, borders and place names. Like any map server, it is asked for the part of the world you are looking at.
        </li>
      </ul>
      <p>
        Nothing else. Our fonts come from our own site. Event data from USGS, NASA EONET, the National Weather Service
        and GDACS reaches you through our API; your browser contacts those agencies only if you follow a link to one.
      </p>

      <h2>Near me</h2>
      <p>
        If you press Near me, your browser asks whether to share your position with the page. The page uses it there, to
        frame the map and sort the list, and forgets it when you leave. It is never sent to us or to anyone else.
      </p>

      <h2>The waiting list</h2>
      <p>
        If you join the waiting list, we keep your email address and the choices you made, to tell you when alerts are
        ready and, only if you asked, to send product news. The list is kept by our email provider, Resend. Every email
        to the list has a link to leave it.{PRIVACY_EMAIL && <> To have your details deleted, write to <Mail />.</>}
      </p>

      <h2>Cookies</h2>
      <p>None. Nothing is stored in your browser, so there is nothing to agree to.</p>

      <h2>Changes</h2>
      <p>
        This page lives in the project&apos;s public repository, so <a href={HISTORY}>every change to it is on record</a>.
      </p>
    </TextPage>
  );
}
