import { Button } from "@/components/ui/button";
import { FEATURED_SYSTEM_INTEGRATIONS } from "@/features/integrations/constants";
import type { IntegrationProvider } from "@/types";

function authLabel(authType: IntegrationProvider["auth_type"]) {
  if (authType === "oauth2") return "OAuth";
  if (authType === "api_key") return "API key";
  return "Credentials";
}

export function SystemIntegrationsCatalog({
  providers,
  onConnect,
}: {
  providers: IntegrationProvider[];
  onConnect: (slug: string) => void;
}) {
  const bySlug = new Map(providers.map((p) => [p.slug, p]));
  const featured = FEATURED_SYSTEM_INTEGRATIONS.map((slug) => bySlug.get(slug)).filter(
    (p): p is IntegrationProvider => p != null
  );

  if (featured.length === 0) return null;

  return (
    <div className="mb-8">
      <h2 className="mb-3 text-sm font-semibold text-gray-700">Available integrations</h2>
      <div className="grid gap-3 sm:grid-cols-2">
        {featured.map((p) => (
          <div
            key={p.slug}
            className="flex flex-col rounded-lg border border-gray-100 bg-white p-4 shadow-sm"
          >
            <div className="mb-1 font-semibold text-gray-900">{p.name}</div>
            <p className="mb-3 flex-1 text-sm text-gray-500">
              {p.description || "Connect this CRM to receive routed leads."}
            </p>
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs text-gray-400">{authLabel(p.auth_type)}</span>
              <Button size="sm" onClick={() => onConnect(p.slug)}>
                Connect
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
