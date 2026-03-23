import type { MetadataRoute } from "next";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: ["/", "/privacy", "/terms", "/waitlist"],
        disallow: ["/auth/", "/api/", "/trips/", "/settings", "/companion", "/shared/"],
      },
    ],
    sitemap: "https://toqui.travel/sitemap.xml",
  };
}
