import type { Metadata } from "next";
import { Inter, Lora } from "next/font/google";
import "./globals.css";

// next/font self-hosts at build time — no request ever reaches Google.
// Inter carries body and UI; Lora carries display and headings.
const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
  display: "swap",
});

const lora = Lora({
  variable: "--font-lora",
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "SentryAtlas — Real-Time Disaster Monitoring",
  description:
    "Open-source platform aggregating real-time disaster data from USGS, NASA EONET, NOAA, and GDACS onto a single interactive map.",
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
      <body className={`${inter.variable} ${lora.variable} antialiased`}>
        {children}
      </body>
    </html>
  );
}
