import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Toqui — AI Travel Companion",
    short_name: "Toqui",
    start_url: "/trips",
    display: "standalone",
    background_color: "#ffffff",
    theme_color: "#e8654a",
    icons: [{ src: "/icon.svg", sizes: "any", type: "image/svg+xml" }],
  };
}
