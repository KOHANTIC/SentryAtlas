import type { Metadata } from "next";
import Link from "next/link";
import AlertsForm from "@/components/AlertsForm";
import { SiteFooter, SiteHeader } from "@/components/Site";
import { Figures, SourceCounts, Thumbnail } from "./HomeLive";

export const metadata: Metadata = { alternates: { canonical: "https://sentryatlas.com/" } };

/** The home page: the sheet's title block, as round 1 chose (#29). */
export default function Home() {
  return (
    <>
      <SiteHeader />
      <main id="main" className="home">
        <section className="intro" aria-labelledby="home-title">
          <span className="label">Today&apos;s sheet</span>
          <h1 id="home-title">What the world&apos;s hazard services are reporting, on one map.</h1>
          <Figures />
          <p>
            <Link href="/map/" className="button primary">Open the map</Link>
          </p>
          <p className="muted">Free, no account. Each source is checked every 2 to 10 minutes.</p>
        </section>
        <Thumbnail />
        <section className="home-alerts" aria-labelledby="alerts-title">
          <h2 id="alerts-title">Get alerts for your area</h2>
          <p>Location alerts aren&apos;t ready yet. Join the waiting list and we&apos;ll email you when they are.</p>
          <AlertsForm />
        </section>
        <section className="home-sources" aria-labelledby="sources-title">
          <h2 id="sources-title">Sources on this sheet</h2>
          <SourceCounts />
          <p className="muted">
            Events in the last 7 days. <Link href="/data/">Where they come from</Link>. SentryAtlas is not an official
            warning service.
          </p>
        </section>
      </main>
      <SiteFooter />
    </>
  );
}
