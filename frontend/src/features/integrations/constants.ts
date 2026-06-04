export const FEATURED_SYSTEM_INTEGRATIONS = [
  "pipedrive",
  "ghl",
  "hubspot",
  "salesforce",
] as const;

export type FeaturedIntegrationSlug = (typeof FEATURED_SYSTEM_INTEGRATIONS)[number];
