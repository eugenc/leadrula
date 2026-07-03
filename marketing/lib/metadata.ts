import type { Metadata } from "next";
import { siteUrl } from "./env";

const defaultDescription =
  "Leadrula is the lead distribution platform where publishers source, qualify, and route leads to buyers — and stay in the deal until it closes.";

export function pageMeta(opts: {
  title: string;
  description?: string;
  path: string;
}): Metadata {
  const description = opts.description ?? defaultDescription;
  const url = `${siteUrl()}${opts.path}`;
  const image = `${siteUrl()}/og-marketing.png`;

  return {
    title: opts.title,
    description,
    alternates: { canonical: url },
    openGraph: {
      title: opts.title,
      description,
      url,
      siteName: "Leadrula",
      type: "website",
      images: [{ url: image, width: 1200, height: 630, alt: "Leadrula" }],
    },
    twitter: {
      card: "summary_large_image",
      title: opts.title,
      description,
      images: [image],
    },
  };
}

export const homeMetadata = pageMeta({
  title: "Leadrula — The Lead Distribution Platform Built to Close",
  description: defaultDescription,
  path: "/",
});
