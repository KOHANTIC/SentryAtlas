import type { MetadataRoute } from "next";

export const dynamic = "force-static";

// Until the cutover (#40) the site's headers also say noindex (public/_headers).
export default function robots(): MetadataRoute.Robots {
  return { rules: { userAgent: "*", allow: "/", disallow: "/waitlist/" }, sitemap: "https://sentryatlas.com/sitemap.xml" };
}
