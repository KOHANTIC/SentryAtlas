import type { Metadata } from "next";

export const metadata: Metadata = { title: "Data" };

/** Data: a stub until its issue fills it. */
export default function DataPage() {
  return (
    <main id="main" className="page">
      <h1>Data</h1>
      <p>What each source sends, and how often.</p>
    </main>
  );
}
