import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { FormDrawer } from "@/components/ui/dialog";
import { IntegrationsCatalog } from "@/features/integrations/IntegrationsCatalog";
import { isHiddenIntegrationSlug } from "@/features/integrations/constants";
import { StripeOAuthReturnHandler, StripeSetupDrawer } from "@/features/integrations/StripeIntegration";
import { useAuthStore } from "@/store/authStore";
import {
  useStripeConnectStatus,
  useStripeKeysStatus,
} from "@/features/admin/hooks";
import {
  useIntegrationProviders,
  useIntegrationConnections,
  useCreateIntegrationConnection,
  useDeleteIntegrationConnection,
  useOAuthConnect,
} from "@/features/integrations/hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { IntegrationConnection, IntegrationProvider } from "@/types";

export function IntegrationsConnectionsTab() {
  const [params] = useSearchParams();
  const isPublisher = useAuthStore((s) => s.user?.account_type === "publisher");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [stripeDrawerOpen, setStripeDrawerOpen] = useState(false);
  const [preselectedSlug, setPreselectedSlug] = useState<string | undefined>();

  const openStripeDrawer = useCallback(() => setStripeDrawerOpen(true), []);

  const { data: providers } = useIntegrationProviders();
  const { data: connections } = useIntegrationConnections();
  const { data: stripeConnect } = useStripeConnectStatus();
  const { data: stripeKeys } = useStripeKeysStatus();

  const visibleConnections = useMemo(
    () => (connections ?? []).filter((c) => !isHiddenIntegrationSlug(c.provider_slug)),
    [connections]
  );

  const activeSlugs = useMemo(() => {
    const slugs = new Set<string>();
    for (const c of visibleConnections) {
      if (c.status === "active") slugs.add(c.provider_slug);
    }
    return slugs;
  }, [visibleConnections]);

  const stripeActive =
    isPublisher &&
    (stripeConnect?.status === "active" || stripeKeys?.status === "verified");

  useEffect(() => {
    const connected = params.get("connected");
    if (connected) {
      toast.success(`Connected ${connected}`);
    }
  }, [params]);

  function openDrawer(slug?: string) {
    setPreselectedSlug(slug);
    setDrawerOpen(true);
  }

  function closeDrawer() {
    setDrawerOpen(false);
    setPreselectedSlug(undefined);
  }

  return (
    <>
      <StripeOAuthReturnHandler onReturn={openStripeDrawer} />

      <IntegrationsCatalog
        providers={providers ?? []}
        isPublisher={isPublisher}
        activeSlugs={activeSlugs}
        stripeActive={stripeActive}
        onManage={(slug) => openDrawer(slug)}
        onStripeConnect={openStripeDrawer}
        onAddIntegration={() => openDrawer()}
      />

      <StripeSetupDrawer open={stripeDrawerOpen} onClose={() => setStripeDrawerOpen(false)} />

      <AddConnectionDrawer
        open={drawerOpen}
        onClose={closeDrawer}
        providers={providers ?? []}
        connections={visibleConnections}
        initialSlug={preselectedSlug}
      />
    </>
  );
}

function AddConnectionDrawer({
  open,
  onClose,
  providers,
  connections,
  initialSlug,
}: {
  open: boolean;
  onClose: () => void;
  providers: IntegrationProvider[];
  connections: IntegrationConnection[];
  initialSlug?: string;
}) {
  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [locationId, setLocationId] = useState("");
  const [apiDomain, setApiDomain] = useState("com");

  const create = useCreateIntegrationConnection();
  const oauth = useOAuthConnect();
  const remove = useDeleteIntegrationConnection();
  const effectiveSlug = slug || initialSlug || "";
  const selected = providers.find((p) => p.slug === effectiveSlug);
  const existingForSlug = connections.filter((c) => c.provider_slug === effectiveSlug);
  const isManage = existingForSlug.length > 0;
  const drawerTitle = isManage && selected ? `Manage ${selected.name}` : "Add Integration";

  useEffect(() => {
    if (!open) {
      setSlug("");
      setName("");
      setApiKey("");
      setLocationId("");
      return;
    }
    if (initialSlug) {
      setSlug(initialSlug);
      const p = providers.find((x) => x.slug === initialSlug);
      if (p) setName(`${p.name} connection`);
    }
  }, [open, initialSlug, providers]);

  function submit() {
    if (!slug || !name) return;
    if (selected?.auth_type === "oauth2") {
      oauth.mutate(
        { provider: slug, name, config: slug === "zoho_crm" ? { api_domain: apiDomain } : {} },
        {
          onSuccess: (authUrl) => {
            window.location.href = authUrl;
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
      return;
    }
    let credentials: Record<string, unknown> = {};
    const config: Record<string, unknown> = {};
    if (slug === "ghl") {
      credentials = { api_key: apiKey };
      config.location_id = locationId;
    }
    create.mutate(
      { provider_slug: slug, name, credentials, config },
      {
        onSuccess: () => {
          toast.success("Connection created");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function disconnect(id: number) {
    remove.mutate(id, {
      onSuccess: () => toast.success("Disconnected"),
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  return (
    <FormDrawer
      open={open}
      onClose={onClose}
      title={drawerTitle}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={!slug || !name || create.isPending || oauth.isPending} onClick={submit}>
            {selected?.auth_type === "oauth2" ? "Connect" : "Save"}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        {isManage && (
          <div className="space-y-2 border-b border-gray-100 pb-4">
            <p className="text-sm font-semibold text-gray-800">Connected</p>
            {existingForSlug.map((c) => (
              <div key={c.id} className="flex items-center justify-between gap-2">
                <span className="text-sm text-gray-700">{c.name}</span>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={remove.isPending}
                  onClick={() => disconnect(c.id)}
                >
                  Disconnect
                </Button>
              </div>
            ))}
          </div>
        )}
        <div>
          <Label>Provider</Label>
          <Select value={slug} onChange={(e) => setSlug(e.target.value)}>
            <option value="">Select…</option>
            {providers
              .filter(
                (p) =>
                  (p.direction === "outbound" || p.direction === "both") &&
                  !isHiddenIntegrationSlug(p.slug)
              )
              .map((p) => (
                <option key={p.slug} value={p.slug}>
                  {p.name}
                </option>
              ))}
          </Select>
        </div>
        <div>
          <Label>Connection name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="My CRM" />
        </div>
        {slug === "ghl" && (
          <>
            <div>
              <Label>API key</Label>
              <Input type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} />
            </div>
            <div>
              <Label>Location ID</Label>
              <Input value={locationId} onChange={(e) => setLocationId(e.target.value)} />
            </div>
          </>
        )}
        {slug === "zoho_crm" && (
          <div>
            <Label>Zoho data center</Label>
            <Select value={apiDomain} onChange={(e) => setApiDomain(e.target.value)}>
              <option value="com">US (.com)</option>
              <option value="eu">EU</option>
              <option value="in">India</option>
              <option value="com.au">Australia</option>
              <option value="jp">Japan</option>
            </Select>
          </div>
        )}
        {selected?.auth_type === "oauth2" && slug !== "zoho_crm" && (
          <p className="text-sm text-gray-400">
            You will be redirected to {selected.name} to authorize access.
          </p>
        )}
      </div>
    </FormDrawer>
  );
}
