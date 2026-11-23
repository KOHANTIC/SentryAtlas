import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Privacy · SentryAtlas",
  description:
    "What SentryAtlas counts, what its servers write down, and which other services your browser contacts.",
  alternates: { canonical: "https://sentryatlas.com/privacy/" },
};

// Read at build time, so a copy of this project never publishes the
// original operator's name or address as its own.
const OPERATOR = process.env.SITE_OPERATOR?.trim();
const PRIVACY_EMAIL = process.env.SITE_PRIVACY_EMAIL?.trim();

const UPDATED = "22 November 2026";
const SOURCE_HISTORY =
  "https://github.com/KOHANTIC/SentryAtlas/commits/main/landing/src/app/privacy/page.tsx";

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="font-display text-2xl font-semibold text-foreground">{title}</h2>
      {children}
    </section>
  );
}

function Out({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className="text-accent-400 underline underline-offset-2 hover:no-underline">
      {children}
    </a>
  );
}

/** Privacy page: what is counted, what is logged, and who else the browser talks to. */
export default function Privacy() {
  return (
    <div className="min-h-screen">
      <nav className="border-b border-border">
        <div className="max-w-3xl mx-auto px-6 h-16 flex items-center">
          <Link href="/" className="text-lg font-bold tracking-tight text-foreground">
            Sentry<span className="text-accent-400">Atlas</span>
          </Link>
        </div>
      </nav>

      <main className="max-w-3xl mx-auto px-6 py-16 space-y-10 text-foreground-muted leading-relaxed">
        <header className="space-y-4">
          <h1 className="font-display text-4xl sm:text-5xl font-semibold text-foreground">Privacy</h1>
          <p className="text-lg">
            SentryAtlas has no ads, no tracking cookies and no accounts. This page says what we
            count, what our servers write down, and which other services your browser contacts.
          </p>
          <p className="text-sm text-foreground-faint">Last changed {UPDATED}.</p>
        </header>

        <Section title="Who runs it">
          {OPERATOR ? (
            <p>
              SentryAtlas at sentryatlas.com is run by {OPERATOR}.
              {PRIVACY_EMAIL && (
                <>
                  {" "}Questions about privacy go to{" "}
                  <a href={`mailto:${PRIVACY_EMAIL}`} className="text-accent-400 underline underline-offset-2 hover:no-underline">
                    {PRIVACY_EMAIL}
                  </a>
                  .
                </>
              )}
            </p>
          ) : (
            <p>This copy of SentryAtlas does not name its operator.</p>
          )}
        </Section>

        <Section title="What we count">
          <p>
            We count visits with{" "}
            <Out href="https://www.cloudflare.com/web-analytics/">Cloudflare Web Analytics</Out>.
            For each page view it records the page, the site that linked to it, your country,
            browser, operating system and type of device, and how fast the page loaded. It does
            not record the part of the address after a question mark.
          </p>
          <p>
            It sets no cookies and stores nothing in your browser. Cloudflare says it does not
            fingerprint visitors or follow them from site to site. We see totals, not people.
            Cloudflare keeps full detail for 7 days and a sample after that; we can look back six
            months.
          </p>
        </Section>

        <Section title="What our servers write down">
          <p>
            For each request, the API logs the method, the path, the status, the size, how long it
            took and a random request ID. It does not log your IP address, your browser, or the part of the address
            after a question mark. Each hour it also counts calls by format, and by whether they
            came from our own pages, another site, or neither.
          </p>
          <p>
            To apply its rate limit, the API keeps your IP address in memory for up to two minutes.
            It never writes it down.
          </p>
          <p>
            Every request passes through{" "}
            <Out href="https://www.cloudflare.com/privacypolicy/">Cloudflare</Out> and{" "}
            <Out href="https://www.digitalocean.com/legal/privacy-policy">DigitalOcean</Out>, which
            host the sites. Like any host they see your IP address, and their own policies cover
            what they keep.
          </p>
        </Section>

        <Section title="Who else your browser talks to">
          <ul className="list-disc pl-6 space-y-2">
            <li>
              <span className="text-foreground">Cloudflare</span> delivers both sites, and the
              analytics script from static.cloudflareinsights.com.
            </li>
            <li>
              <span className="text-foreground">
                <Out href="https://openfreemap.org/">OpenFreeMap</Out>
              </span>{" "}
              (tiles.openfreemap.org) draws the map underneath the events: land, borders and
              place names. Like any map server, it is asked for the part of the world you are
              looking at.
            </li>
          </ul>
          <p>
            Nothing else. Our fonts come from our own sites. Event data from USGS, NASA EONET, the
            National Weather Service and GDACS reaches you through our API; your browser contacts
            those agencies only if you follow a link to one.
          </p>
        </Section>

        <Section title="Cookies">
          <p>None. Nothing is stored in your browser, so there is nothing to agree to.</p>
        </Section>

        <Section title="Changes">
          <p>
            This page lives in the project&apos;s public repository, so{" "}
            <Out href={SOURCE_HISTORY}>every change to it is on record</Out>.
          </p>
        </Section>
      </main>

      <footer className="py-10 px-6 border-t border-border">
        <div className="max-w-3xl mx-auto flex items-center gap-6 text-sm text-foreground-muted">
          <Link href="/" className="hover:text-foreground transition-colors">Home</Link>
          <a href="https://map.sentryatlas.com" className="hover:text-foreground transition-colors">Map</a>
        </div>
      </footer>
    </div>
  );
}
