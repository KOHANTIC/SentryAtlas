import Link from "next/link";

const GITHUB = "https://github.com/KOHANTIC/SentryAtlas";

/** The sheet mark (brand/logo), in the theme's colors. */
export function Logo({ size = 24 }: { size?: number }) {
  return (
    <svg viewBox="0 0 26 26" width={size} height={size} aria-hidden="true">
      <rect x="1" y="1" width="24" height="24" fill="none" stroke="var(--ink)" strokeWidth="2" />
      <path d="M4 17c4-5 7-1 10-5s5-4 8-2" fill="none" stroke="var(--action)" strokeWidth="2" />
      <circle cx="17" cy="9" r="2.4" fill="var(--ink)" />
    </svg>
  );
}

/** The header of every page but the map. */
export function SiteHeader() {
  return (
    <header className="site-header">
      <Link href="/" className="wordmark">
        <Logo />
        SentryAtlas
      </Link>
      <nav aria-label="Site">
        <Link href="/map/">Map</Link>
        <Link href="/data/">Sources</Link>
        <Link href="/developers/">Developers</Link>
        <Link href="/alerts/">Get alerts</Link>
      </nav>
    </header>
  );
}

/** The footer of every page but the map. */
export function SiteFooter() {
  return (
    <footer className="site-footer">
      <span>Made by the KOHANTIC team</span>
      <nav aria-label="About">
        <Link href="/privacy/">Privacy</Link>
        <Link href="/terms/">Terms</Link>
        {/* AGPL §13: a network service offers its users its source. */}
        <a href={GITHUB}>Source</a>
      </nav>
    </footer>
  );
}

/** A plain text page: header, a title block, the content, footer. */
export function TextPage({ label, title, children }: { label: string; title: string; children: React.ReactNode }) {
  return (
    <>
      <SiteHeader />
      <main id="main" className="text-page">
        <span className="label">{label}</span>
        <h1>{title}</h1>
        {children}
      </main>
      <SiteFooter />
    </>
  );
}

