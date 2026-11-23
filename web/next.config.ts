import path from "node:path";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Plain files served by Cloudflare Workers static assets (#11): no server.
  output: "export",
  // Each page is a folder with an index.html, which every static host serves.
  trailingSlash: true,
  images: { unoptimized: true },
  // The design tokens live in ../brand, outside this app, so the build's root is the repo.
  turbopack: { root: path.resolve("..") },
};

export default nextConfig;
