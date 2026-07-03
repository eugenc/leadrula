import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { MarketingShell } from "@/components/MarketingShell";
import { homeMetadata } from "@/lib/metadata";
import { siteUrl } from "@/lib/env";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
});

export const metadata: Metadata = {
  ...homeMetadata,
  metadataBase: new URL(siteUrl()),
  title: {
    default: "Leadrula — The Lead Distribution Platform Built to Close",
    template: "%s | Leadrula",
  },
};

const orgJsonLd = {
  "@context": "https://schema.org",
  "@type": "Organization",
  name: "Leadrula",
  url: siteUrl(),
  logo: `${siteUrl()}/leadrula-logo-light.png`,
  description:
    "Lead distribution platform where publishers and buyers source, qualify, route, and close leads together.",
  parentOrganization: {
    "@type": "Organization",
    name: "Affiniti",
  },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${inter.variable} h-full`}>
      <body className="min-h-full">
        <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(orgJsonLd) }} />
        <MarketingShell>{children}</MarketingShell>
      </body>
    </html>
  );
}
