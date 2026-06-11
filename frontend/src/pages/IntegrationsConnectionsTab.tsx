import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useSearchParams } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { FormDrawer } from "@/components/ui/dialog";
import { IntegrationsCatalog } from "@/features/integrations/IntegrationsCatalog";
import {
  integrationLogoClassName,
  integrationLogoUrl,
  isHiddenIntegrationSlug,
} from "@/features/integrations/constants";
import { SunbaseFieldMapSection } from "@/features/integrations/SunbaseFieldMapSection";
import { SunbaseInboundEndpointSection } from "@/features/integrations/SunbaseInboundEndpointSection";
import {
  SUNBASE_URL,
  sunbaseFieldMap,
  syncSchemaInFieldMap,
} from "@/features/integrations/sunbaseConstants";
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
  useTestIntegrationConnection,
  useUpdateIntegrationConnection,
  useSunbaseConnectionDetail,
} from "@/features/integrations/hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { IntegrationConnection, IntegrationProvider, OutboundFieldMapEntry, SunbaseInboundWebhook } from "@/types";

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

  const [endpointUrl, setEndpointUrl] = useState(SUNBASE_URL);
  const [schemaName, setSchemaName] = useState("");
  const [fieldMap, setFieldMap] = useState<OutboundFieldMapEntry[]>(sunbaseFieldMap(""));
  const [sunbaseActive, setSunbaseActive] = useState(false);
  const [activeConnection, setActiveConnection] = useState<IntegrationConnection | null>(null);
  const [inboundWebhook, setInboundWebhook] = useState<SunbaseInboundWebhook | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);

  const create = useCreateIntegrationConnection();
  const update = useUpdateIntegrationConnection();
  const testConn = useTestIntegrationConnection();
  const oauth = useOAuthConnect();
  const remove = useDeleteIntegrationConnection();

  const effectiveSlug = slug || initialSlug || "";
  const selected = providers.find((p) => p.slug === effectiveSlug);
  const existingForSlug = connections.filter((c) => c.provider_slug === effectiveSlug);
  const isSunbase = effectiveSlug === "sunbase";
  const isManage = existingForSlug.length > 0;
  const showSunbaseActive = isSunbase && sunbaseActive && activeConnection != null;

  const detailId = showSunbaseActive ? activeConnection.id : null;
  const { data: sunbaseDetail } = useSunbaseConnectionDetail(detailId);

  const drawerTitle = showSunbaseActive
    ? "SunBase connected"
    : isManage && selected
      ? `Manage ${selected.name}`
      : "Add Integration";

  const accountType = useAuthStore((s) => s.user?.account_type);
  const webhooksPath = accountType === "publisher" ? "/p/webhooks" : "/b/webhooks";

  useEffect(() => {
    if (!open) {
      setSlug("");
      setName("");
      setApiKey("");
      setLocationId("");
      setEndpointUrl(SUNBASE_URL);
      setSchemaName("");
      setFieldMap(sunbaseFieldMap(""));
      setSunbaseActive(false);
      setActiveConnection(null);
      setInboundWebhook(null);
      setShowAdvanced(false);
      return;
    }
    if (initialSlug) {
      setSlug(initialSlug);
      const p = providers.find((x) => x.slug === initialSlug);
      if (p) setName(`${p.name} connection`);
      if (initialSlug === "sunbase") {
        const existing = connections.find((c) => c.provider_slug === "sunbase");
        if (existing) {
          loadSunbaseConnection(existing);
        }
      }
    }
  }, [open, initialSlug, providers, connections]);

  useEffect(() => {
    if (schemaName) {
      setFieldMap((prev) => syncSchemaInFieldMap(prev, schemaName));
    }
  }, [schemaName]);

  useEffect(() => {
    if (sunbaseDetail?.inbound_webhook) {
      setInboundWebhook(sunbaseDetail.inbound_webhook);
    }
  }, [sunbaseDetail]);

  function loadSunbaseConnection(conn: IntegrationConnection) {
    setActiveConnection(conn);
    setSunbaseActive(true);
    setName(conn.name);
    const cfg = conn.config ?? {};
    if (typeof cfg.endpoint_url === "string") setEndpointUrl(cfg.endpoint_url);
    const map = cfg.outbound_field_map as OutboundFieldMapEntry[] | undefined;
    if (map?.length) {
      setFieldMap(map);
      const schemaEntry = map.find(
        (e) => e.dest_key === "schema_name" && e.source_type === "static"
      );
      if (schemaEntry?.static_value) setSchemaName(schemaEntry.static_value);
    }
    if (conn.inbound_webhook) setInboundWebhook(conn.inbound_webhook);
  }

  function runTestConnection() {
    if (!schemaName.trim() || !endpointUrl.trim()) return;
    testConn.mutate(
      {
        provider_slug: "sunbase",
        credentials: { schema_name: schemaName.trim() },
        config: { endpoint_url: endpointUrl.trim(), outbound_field_map: fieldMap },
      },
      {
        onSuccess: (res) => {
          if (res.ok) toast.success("Connection successful");
          else toast.error(res.message ?? "Connection failed");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function submit() {
    if (!slug) return;
    if (!isSunbase && !name) return;
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
    if (slug === "sunbase") {
      if (!schemaName.trim()) {
        toast.error("Schema name is required");
        return;
      }
      create.mutate(
        {
          provider_slug: "sunbase",
          name: "",
          credentials: { schema_name: schemaName.trim() },
          config: { endpoint_url: endpointUrl.trim(), outbound_field_map: fieldMap },
        },
        {
          onSuccess: (conn) => {
            setActiveConnection(conn);
            setSunbaseActive(true);
            if (conn.inbound_webhook) setInboundWebhook(conn.inbound_webhook);
            toast.success("SunBase connected");
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

  function saveSunbaseChanges() {
    if (!activeConnection) return;
    update.mutate(
      {
        id: activeConnection.id,
        credentials: schemaName.trim() ? { schema_name: schemaName.trim() } : undefined,
        config: { endpoint_url: endpointUrl.trim(), outbound_field_map: fieldMap },
      },
      {
        onSuccess: (conn) => {
          setActiveConnection(conn);
          if (conn.inbound_webhook) setInboundWebhook(conn.inbound_webhook);
          toast.success("Saved");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function disconnect(id: number, closeOnSuccess = false) {
    remove.mutate(id, {
      onSuccess: () => {
        toast.success("Disconnected");
        if (activeConnection?.id === id) {
          setSunbaseActive(false);
          setActiveConnection(null);
          setInboundWebhook(null);
        }
        if (closeOnSuccess) onClose();
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  const logo = integrationLogoUrl("sunbase");

  return (
    <FormDrawer
      open={open}
      onClose={onClose}
      title={drawerTitle}
      footer={
        showSunbaseActive ? (
          <>
            {activeConnection && (
              <Button
                variant="secondary"
                disabled={remove.isPending}
                onClick={() => disconnect(activeConnection.id, true)}
              >
                Disconnect
              </Button>
            )}
            <Button variant="secondary" onClick={onClose}>
              Done
            </Button>
            <Button disabled={update.isPending} onClick={saveSunbaseChanges}>
              Save changes
            </Button>
          </>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button
              disabled={
                !slug ||
                (!isSunbase && !name) ||
                (isSunbase && !schemaName.trim()) ||
                create.isPending ||
                oauth.isPending
              }
              onClick={submit}
            >
              {selected?.auth_type === "oauth2" ? "Connect" : "Save"}
            </Button>
          </>
        )
      }
    >
      <div className="space-y-4">
        {isSunbase && logo && (
          <div className="flex items-center gap-2">
            <img src={logo} alt="" className={integrationLogoClassName("sunbase")} />
            <span className="text-sm text-gray-500">SunBase CRM</span>
          </div>
        )}
        {showSunbaseActive && inboundWebhook && (
          <SunbaseInboundEndpointSection inbound={inboundWebhook} />
        )}

        {isManage && !showSunbaseActive && (
          <div className="space-y-2 border-b border-gray-100 pb-4">
            <p className="text-sm font-semibold text-gray-800">Connected</p>
            {existingForSlug.map((c) => (
              <div key={c.id} className="flex items-center justify-between gap-2">
                <button
                  type="button"
                  className="text-left text-sm text-indigo-600 hover:underline"
                  onClick={() => isSunbase && loadSunbaseConnection(c)}
                >
                  {c.name}
                </button>
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

        <>
            {!showSunbaseActive && (
              <>
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
                {slug !== "sunbase" && (
                  <div>
                    <Label>Connection name</Label>
                    <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="My CRM" />
                  </div>
                )}
              </>
            )}

            {isSunbase && (slug || showSunbaseActive) && (
              <div className="space-y-3">
                {!showSunbaseActive && (
                  <p className="text-xs text-amber-700">
                    Test connection may create a test lead in SunBase (last_name: ConnectionTest).
                  </p>
                )}
                <div>
                  <Label>Endpoint URL</Label>
                  <Input
                    value={endpointUrl}
                    onChange={(e) => setEndpointUrl(e.target.value)}
                    placeholder={SUNBASE_URL}
                  />
                </div>
                <div>
                  <Label>Schema name</Label>
                  <Input
                    value={schemaName}
                    onChange={(e) => setSchemaName(e.target.value)}
                    placeholder="From your SunBase provider"
                  />
                </div>
                <div className="flex gap-2">
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={!schemaName.trim() || !endpointUrl.trim() || testConn.isPending}
                    onClick={runTestConnection}
                  >
                    Test connection
                  </Button>
                </div>

                {(showSunbaseActive || showAdvanced) && (
                  <SunbaseFieldMapSection entries={fieldMap} onChange={setFieldMap} />
                )}

                {showSunbaseActive && (
                  <>
                    <p className="text-xs text-amber-700">
                      Only enable outbound triggers on the POST or GET webhook, not both — otherwise
                      leads may be sent twice.
                    </p>
                    <p className="text-xs text-gray-500">
                      Outbound webhooks are on the{" "}
                      <Link to={webhooksPath} className="text-indigo-600 hover:underline">
                        Webhooks page
                      </Link>
                      .
                    </p>
                  </>
                )}

                {!showSunbaseActive && (
                  <button
                    type="button"
                    className="text-xs text-indigo-600 hover:underline"
                    onClick={() => setShowAdvanced((v) => !v)}
                  >
                    {showAdvanced ? "Hide field mapping" : "Customize field mapping"}
                  </button>
                )}
              </div>
            )}

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
        </>
      </div>
    </FormDrawer>
  );
}
