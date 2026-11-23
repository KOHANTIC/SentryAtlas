import type { Metadata } from "next";
import EmbedPage from "./EmbedPage";

// Other sites show this page in a frame; search engines should send people to /map/ instead.
export const metadata: Metadata = { title: "Embedded map", robots: { index: false } };

export default function Page() {
  return <EmbedPage />;
}
