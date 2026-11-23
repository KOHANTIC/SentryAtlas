import Link from "next/link";

/** The frame of a plain text page: the wordmark, the content, and a short footer. */
export default function SimplePage({ children }: { children: React.ReactNode }) {
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
        {children}
      </main>

      <footer className="py-10 px-6 border-t border-border">
        <div className="max-w-3xl mx-auto flex items-center gap-6 text-sm text-foreground-muted">
          <Link href="/" className="hover:text-foreground transition-colors">Home</Link>
          <a href="https://map.sentryatlas.com" className="hover:text-foreground transition-colors">Map</a>
          <Link href="/privacy/" className="hover:text-foreground transition-colors">Privacy</Link>
        </div>
      </footer>
    </div>
  );
}
