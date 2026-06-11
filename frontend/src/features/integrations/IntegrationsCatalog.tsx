import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge, EmptyState } from "@/components/ui/misc";
import { cn } from "@/lib/utils";
import {
  INTEGRATION_CATEGORY,
  INTEGRATION_FILTERS,
  INTEGRATIONS_PAGE_SIZE,
  STRIPE_INTEGRATION,
  integrationLogoUrl,
  integrationLogoClassName,
  isHiddenIntegrationSlug,
  type IntegrationCategory,
  type IntegrationFilter,
} from "@/features/integrations/constants";
import type { IntegrationProvider } from "@/types";

function authLabel(authType: IntegrationProvider["auth_type"]) {
  if (authType === "oauth2") return "OAuth";
  if (authType === "api_key") return "API key";
  return "Credentials";
}

function matchesSearch(name: string, description: string, query: string) {
  if (!query) return true;
  const haystack = `${name} ${description}`.toLowerCase();
  return haystack.includes(query);
}

function matchesFilter(
  category: IntegrationCategory | undefined,
  filter: IntegrationFilter,
  isPublisher: boolean
) {
  if (filter === "all") return true;
  if (filter === "payment") return isPublisher && category === "payment";
  if (filter === "crm") return category === "crm";
  return false;
}

type CatalogItem =
  | { kind: "provider"; provider: IntegrationProvider }
  | { kind: "stripe" };

function IntegrationCatalogCard({
  slug,
  name,
  description,
  authLabel: auth,
  isActive,
  onAction,
  actionLabel,
}: {
  slug: string;
  name: string;
  description: string;
  authLabel: string;
  isActive: boolean;
  onAction: () => void;
  actionLabel: string;
}) {
  const logo = integrationLogoUrl(slug);
  return (
    <div className="aspect-[2/1] w-full overflow-hidden rounded-lg border border-gray-100 bg-surface-card shadow-xs">
      <div className="flex h-full gap-3 p-3">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <div className="mb-1 flex flex-wrap items-center gap-2">
            <div className="font-semibold text-gray-900">{name}</div>
            {isActive && <Badge variant="distributed">Active</Badge>}
          </div>
          <p className="mb-auto line-clamp-2 text-sm text-gray-500">{description}</p>
          <span className="mt-1 text-xs text-gray-400">{auth}</span>
        </div>
        <div className="flex shrink-0 flex-col items-end justify-between gap-2">
          {logo ? (
            <img
              src={logo}
              alt=""
              className={integrationLogoClassName(slug)}
            />
          ) : (
            <div className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-md bg-gray-100 text-xs font-semibold text-gray-500">
              {name.charAt(0)}
            </div>
          )}
          <Button size="sm" className="shrink-0 whitespace-nowrap" onClick={onAction}>
            {actionLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}

export function IntegrationsCatalog({
  providers,
  isPublisher,
  activeSlugs,
  stripeActive = false,
  onManage,
  onStripeConnect,
  onAddIntegration,
}: {
  providers: IntegrationProvider[];
  isPublisher: boolean;
  activeSlugs: ReadonlySet<string>;
  stripeActive?: boolean;
  onManage: (slug: string) => void;
  onStripeConnect: () => void;
  onAddIntegration: () => void;
}) {
  const [filter, setFilter] = useState<IntegrationFilter>("all");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);

  const query = search.trim().toLowerCase();

  useEffect(() => {
    setPage(1);
  }, [filter, query]);

  const items = useMemo(() => {
    const connectable = providers.filter(
      (p) =>
        (p.direction === "outbound" || p.direction === "both") &&
        !isHiddenIntegrationSlug(p.slug)
    );
    const list: CatalogItem[] = connectable.map((p) => ({ kind: "provider", provider: p }));
    if (isPublisher) {
      list.push({ kind: "stripe" });
    }
    return list.filter((item) => {
      if (item.kind === "stripe") {
        const category = INTEGRATION_CATEGORY.stripe;
        return (
          matchesFilter(category, filter, isPublisher) &&
          matchesSearch(STRIPE_INTEGRATION.name, STRIPE_INTEGRATION.description, query)
        );
      }
      const category = INTEGRATION_CATEGORY[item.provider.slug];
      return (
        matchesFilter(category, filter, isPublisher) &&
        matchesSearch(
          item.provider.name,
          item.provider.description || "",
          query
        )
      );
    });
  }, [providers, isPublisher, filter, query]);

  const total = items.length;
  const totalPages = Math.max(1, Math.ceil(total / INTEGRATIONS_PAGE_SIZE));
  const paginatedItems = items.slice(
    (page - 1) * INTEGRATIONS_PAGE_SIZE,
    page * INTEGRATIONS_PAGE_SIZE
  );

  return (
    <div className="mb-8">
      <div className="mb-4 flex border-b border-gray-100">
        {INTEGRATION_FILTERS.map((f) => (
          <button
            key={f.value}
            type="button"
            onClick={() => setFilter(f.value)}
            className={cn(
              "-mb-px border-b-2 px-4 py-2 text-sm font-semibold transition-colors",
              filter === f.value
                ? "border-jade-500 text-jade-700"
                : "border-transparent text-gray-400 hover:text-gray-600"
            )}
          >
            {f.label}
          </button>
        ))}
      </div>

      <div className="mb-4 flex items-center gap-3">
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search integrations…"
          className="max-w-sm flex-1 text-sm"
        />
        <Button className="shrink-0" onClick={onAddIntegration}>
          Add Integration
        </Button>
      </div>

      {total === 0 ? (
        <EmptyState title="No integrations match your search." />
      ) : (
        <>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
          {paginatedItems.map((item) => {
            if (item.kind === "stripe") {
              return (
                <IntegrationCatalogCard
                  key="stripe"
                  slug={STRIPE_INTEGRATION.slug}
                  name={STRIPE_INTEGRATION.name}
                  description={STRIPE_INTEGRATION.description}
                  authLabel={STRIPE_INTEGRATION.authLabel}
                  isActive={stripeActive}
                  onAction={onStripeConnect}
                  actionLabel={stripeActive ? "Manage" : "Connect"}
                />
              );
            }
            const p = item.provider;
            const isActive = activeSlugs.has(p.slug);
            return (
              <IntegrationCatalogCard
                key={p.slug}
                slug={p.slug}
                name={p.name}
                description={p.description || "Connect this CRM to receive routed leads."}
                authLabel={authLabel(p.auth_type)}
                isActive={isActive}
                onAction={() => onManage(p.slug)}
                actionLabel={isActive ? "Manage" : "Connect"}
              />
            );
          })}
        </div>
        {total > INTEGRATIONS_PAGE_SIZE && (
          <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500">
            <span>
              {(page - 1) * INTEGRATIONS_PAGE_SIZE + 1}–{Math.min(page * INTEGRATIONS_PAGE_SIZE, total)} of {total}
            </span>
            <div className="flex items-center gap-3">
              <Button variant="secondary" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
                Previous
              </Button>
              <span>
                Page {page} of {totalPages}
              </span>
              <Button
                variant="secondary"
                size="sm"
                disabled={page >= totalPages}
                onClick={() => setPage(page + 1)}
              >
                Next
              </Button>
            </div>
          </div>
        )}
        </>
      )}
    </div>
  );
}
