import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";

// next/font self-hosts at build time — no request ever reaches Google.
// The map app is a pure utility surface with no visible headings, so it
// carries Inter only; the landing page pairs it with Lora for display.
const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "SentryAtlas",
  description:
    "Real-time disaster monitoring map aggregating data from USGS, EONET, NOAA, and GDACS",
  icons: {
    icon: [
      { url: "/favicon.ico", sizes: "any" },
      { url: "/favicon-16x16.png", sizes: "16x16", type: "image/png" },
      { url: "/favicon-32x32.png", sizes: "32x32", type: "image/png" },
    ],
    apple: "/apple-touch-icon.png",
  },
  manifest: "/site.webmanifest",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${inter.variable} antialiased`}>{children}</body>
    </html>
  );
}
