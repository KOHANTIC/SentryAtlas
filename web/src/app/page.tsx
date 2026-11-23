import Link from "next/link";
import { PAGES } from "@/lib/pages";

/** The landing page; built in #39. */
export default function Home() {
  return (
    <main id="main" className="page">
      <span className="label">SentryAtlas</span>
      <h1>Every disaster on one map</h1>
      <p>Earthquakes, wildfires, floods and storms from four public agencies.</p>
      <nav aria-label="Pages">
        <ul>
          {PAGES.filter((p) => p.path !== "/").map((p) => (
            <li key={p.path}><Link href={p.path}>{p.title}</Link></li>
          ))}
        </ul>
      </nav>
    </main>
  );
}
