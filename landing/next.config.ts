import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  // The host serves a folder's index.html but not a bare name's .html, so
  // /privacy/ must be a folder.
  trailingSlash: true,
};

export default nextConfig;
