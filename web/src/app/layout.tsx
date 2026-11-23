import type { Metadata, Viewport } from "next";
import { Public_Sans, Sofia_Sans_Extra_Condensed } from "next/font/google";
import "./tokens.css";
import "./globals.css";

// next/font serves both faces from this site at build time: no page loads them from a font host.
const text = Public_Sans({ subsets: ["latin"], weight: ["400", "500", "600", "700"], variable: "--nf-text", display: "swap" });
const label = Sofia_Sans_Extra_Condensed({ subsets: ["latin"], weight: ["600", "700", "800"], variable: "--nf-label", display: "swap" });

export const metadata: Metadata = {
  metadataBase: new URL("https://sentryatlas.com"),
  title: { default: "SentryAtlas", template: "%s · SentryAtlas" },
  description: "Earthquakes, wildfires, floods and storms from four public agencies, on one map.",
  icons: { icon: "/icon.svg" },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f3f6f2" },
    { media: "(prefers-color-scheme: dark)", color: "#11181c" },
  ],
};

/** The root of every page: fonts, tokens, and a skip link. */
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${text.variable} ${label.variable}`}>
      <body>
        <a href="#main" className="skip">Skip to content</a>
        {children}
      </body>
    </html>
  );
}
