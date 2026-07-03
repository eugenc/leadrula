import type { MetadataRoute } from "next";
import { siteUrl } from "@/lib/env";

const routes = [
  "",
  "/platform",
  "/publishers",
  "/buyers",
  "/exchange",
  "/pricing",
  "/contact",
  "/privacy",
  "/terms",
];

export default function sitemap(): MetadataRoute.Sitemap {
  const base = siteUrl();
  return routes.map((path) => ({
    url: `${base}${path}`,
    lastModified: new Date(),
    changeFrequency: path === "" ? "weekly" : "monthly",
    priority: path === "" ? 1 : 0.8,
  }));
}
