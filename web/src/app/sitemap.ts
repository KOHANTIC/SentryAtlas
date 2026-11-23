import type { MetadataRoute } from "next";
import { PAGES } from "@/lib/pages";

export const dynamic = "force-static";

/** Every page, at its address after the cutover (#40). */
export default function sitemap(): MetadataRoute.Sitemap {
  return PAGES.map((p) => p.path).map((path) => ({ url: `https://sentryatlas.com${path}` }));
}
