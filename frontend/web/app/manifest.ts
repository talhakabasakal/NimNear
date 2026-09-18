import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "NIMNear",
    short_name: "NIMNear",
    description: "Discover nearby places and events, and pay with NIM.",
    start_url: "/",
    display: "standalone",
    background_color: "#000000",
    theme_color: "#0f0e10",
    icons: [
      {
        src: "/icon-192x192.png",
        sizes: "192x192",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/icon-512x512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
    ],
  };
}
