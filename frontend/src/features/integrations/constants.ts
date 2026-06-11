export const FEATURED_SYSTEM_INTEGRATIONS = [
  "pipedrive",
  "ghl",
  "hubspot",
  "salesforce",
  "sunbase",
] as const;

export type FeaturedIntegrationSlug = (typeof FEATURED_SYSTEM_INTEGRATIONS)[number];

export type IntegrationCategory = "crm" | "payment";

export type IntegrationFilter = "all" | "crm" | "payment";

export const INTEGRATION_CATEGORY: Record<string, IntegrationCategory> = {
  pipedrive: "crm",
  ghl: "crm",
  hubspot: "crm",
  zoho_crm: "crm",
  salesforce: "crm",
  sunbase: "crm",
  google_calendar: "crm",
  stripe: "payment",
};

export const INTEGRATION_LOGOS: Record<string, string> = {
  ghl: "/integrations/ghl.png",
  hubspot: "/integrations/hubspot.png",
  stripe: "/integrations/stripe.png",
  zoho_crm: "/integrations/zoho_crm.png",
  salesforce: "/integrations/salesforce.png",
  pipedrive: "/integrations/pipedrive.png",
  sunbase: "/integrations/sunbase.png",
  google_calendar: "/integrations/google_calendar.png",
};

export function integrationLogoClassName(slug: string): string {
  if (slug === "sunbase") {
    return "h-[30px] w-[30px] shrink-0 rounded-md border border-gray-100 bg-white object-contain p-0.5";
  }
  return "h-[30px] w-[30px] shrink-0 rounded-md object-cover";
}

export function integrationLogoUrl(slug: string): string | undefined {
  return INTEGRATION_LOGOS[slug];
}

export const STRIPE_INTEGRATION = {
  slug: "stripe",
  name: "Stripe",
  description:
    "Connect Stripe for marketplace payouts and configure API keys for direct buyer billing.",
  authLabel: "OAuth · API key",
} as const;

export const INTEGRATIONS_PAGE_SIZE = 25;

export const INTEGRATION_FILTERS: { value: IntegrationFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "crm", label: "CRM" },
  { value: "payment", label: "Payment" },
];

export const HIDDEN_INTEGRATION_SLUGS = ["webhook"] as const;

export type HiddenIntegrationSlug = (typeof HIDDEN_INTEGRATION_SLUGS)[number];

export function isHiddenIntegrationSlug(slug: string): slug is HiddenIntegrationSlug {
  return (HIDDEN_INTEGRATION_SLUGS as readonly string[]).includes(slug);
}
