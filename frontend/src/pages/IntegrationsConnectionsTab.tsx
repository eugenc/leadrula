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
import { GhlInboundEndpointSection } from "@/features/integrations/GhlInboundEndpointSection";
import { GhlConnectionSettings } from "@/features/integrations/GhlConnectionSettings";
import {
  DEFAULT_GHL_CONFIG,
  normalizeGhlConfig,
  isGhlWebhookMode,
  type GHLConfig,
} from "@/features/integrations/ghlConstants";
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
  useGhlConnectionDetail,
  useGhlPipelines,
  useGhlCalendars,
  useFetchGhlMetadata,
  type GhlMetadataPreview,
} from "@/features/integrations/hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { Spinner } from "@/components/ui/misc";
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
  const [pitToken, setPitToken] = useState("");
  const [locationId, setLocationId] = useState("");
  const [apiDomain, setApiDomain] = useState("com");
  const [twilioSid, setTwilioSid] = useState("");
  const [twilioToken, setTwilioToken] = useState("");

  const [endpointUrl, setEndpointUrl] = useState(SUNBASE_URL);
  const [schemaName, setSchemaName] = useState("");
  const [fieldMap, setFieldMap] = useState<OutboundFieldMapEntry[]>(sunbaseFieldMap(""));
  const [sunbaseActive, setSunbaseActive] = useState(false);
  const [ghlActive, setGhlActive] = useState(false);
  const [ghlConfig, setGhlConfig] = useState<GHLConfig>(DEFAULT_GHL_CONFIG(""));
  const [activeConnection, setActiveConnection] = useState<IntegrationConnection | null>(null);
  const [inboundWebhook, setInboundWebhook] = useState<SunbaseInboundWebhook | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [ghlMetadataPreview, setGhlMetadataPreview] = useState<GhlMetadataPreview | null>(null);

  const create = useCreateIntegrationConnection();
  const update = useUpdateIntegrationConnection();
  const testConn = useTestIntegrationConnection();
  const fetchGhlMetadata = useFetchGhlMetadata();
  const oauth = useOAuthConnect();
  const remove = useDeleteIntegrationConnection();

  const effectiveSlug = slug || initialSlug || "";
  const selected = providers.find((p) => p.slug === effectiveSlug);
  const existingForSlug = connections.filter((c) => c.provider_slug === effectiveSlug);
  const isSunbase = effectiveSlug === "sunbase";
  const isGhl = effectiveSlug === "ghl";
  const isGoogleMaps = effectiveSlug === "google_maps";
  const isTwilio = effectiveSlug === "twilio";
  const isManage = existingForSlug.length > 0;
  const showSunbaseActive = isSunbase && sunbaseActive && activeConnection != null;
  const showGhlActive = isGhl && ghlActive && activeConnection != null;

  const detailId = showSunbaseActive ? activeConnection.id : null;
  const ghlDetailId = showGhlActive ? activeConnection.id : null;
  const { data: sunbaseDetail } = useSunbaseConnectionDetail(detailId);
  const { data: ghlDetail } = useGhlConnectionDetail(ghlDetailId);
  const { data: ghlPipelinesData, isLoading: ghlPipelinesLoading } = useGhlPipelines(ghlDetailId);
  const { data: ghlCalendarsData, isLoading: ghlCalendarsLoading } = useGhlCalendars(ghlDetailId);

  const ghlPipelines =
    ghlPipelinesData?.pipelines ?? ghlMetadataPreview?.pipelines ?? [];
  const ghlCalendars =
    ghlCalendarsData?.calendars ?? ghlMetadataPreview?.calendars ?? [];
  const ghlPipelinesBusy = showGhlActive ? ghlPipelinesLoading : fetchGhlMetadata.isPending;
  const ghlCalendarsBusy = showGhlActive ? ghlCalendarsLoading : fetchGhlMetadata.isPending;

  const drawerTitle = showSunbaseActive
    ? "SunBase connected"
    : showGhlActive
      ? "GoHighLevel connected"
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
      setPitToken("");
      setLocationId("");
      setEndpointUrl(SUNBASE_URL);
      setSchemaName("");
      setFieldMap(sunbaseFieldMap(""));
      setSunbaseActive(false);
      setGhlActive(false);
      setGhlConfig(DEFAULT_GHL_CONFIG(""));
      setActiveConnection(null);
      setInboundWebhook(null);
      setShowAdvanced(false);
      setGhlMetadataPreview(null);
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
      if (initialSlug === "ghl") {
        const existing = connections.find((c) => c.provider_slug === "ghl");
        if (existing) {
          loadGhlConnection(existing);
        }
      }
      if (initialSlug === "google_maps") {
        setName("Google Maps");
      }
    }
  }, [open, initialSlug, providers, connections]);

  useEffect(() => {
    if (showGhlActive) return;
    setGhlMetadataPreview(null);
  }, [pitToken, locationId, showGhlActive]);

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

  useEffect(() => {
    if (ghlDetail?.inbound_webhook) {
      setInboundWebhook(ghlDetail.inbound_webhook);
    }
  }, [ghlDetail]);

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

  function loadGhlConnection(conn: IntegrationConnection) {
    setActiveConnection(conn);
    setGhlActive(true);
    setSunbaseActive(false);
    setName(conn.name);
    const cfg = (conn.config ?? {}) as GHLConfig;
    setLocationId(cfg.location_id ?? "");
    setGhlConfig(normalizeGhlConfig({
      ...DEFAULT_GHL_CONFIG(cfg.location_id ?? ""),
      ...cfg,
      create_contact: true,
    }));
    if (conn.inbound_webhook) setInboundWebhook(conn.inbound_webhook);
  }

  const ghlWebhookMode = isGhlWebhookMode(ghlConfig);

  function runTestGhlConnection() {
    if (ghlWebhookMode) {
      if (!ghlConfig.webhook_url?.trim()) {
        toast.error("GHL automation webhook URL is required");
        return;
      }
      testConn.mutate(
        {
          provider_slug: "ghl",
          credentials: {},
          config: normalizeGhlConfig({ ...ghlConfig, create_contact: true }),
        },
        {
          onSuccess: (res) => {
            if (res.ok) toast.success("Webhook test sent");
            else toast.error(res.message ?? "Connection failed");
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
      return;
    }
    if (!pitToken.trim()) {
      toast.error("Enter a Private Integration Token to test");
      return;
    }
    if (!locationId.trim()) {
      toast.error("Location ID is required");
      return;
    }
    testConn.mutate(
      {
        provider_slug: "ghl",
        credentials: { private_integration_token: pitToken.trim() },
        config: { ...ghlConfig, location_id: locationId.trim() },
      },
      {
        onSuccess: (res) => {
          if (!res.ok) {
            toast.error(res.message ?? "Connection failed");
            return;
          }
          toast.success("Connection successful");
          fetchGhlMetadata.mutate(
            {
              credentials: { private_integration_token: pitToken.trim() },
              config: { location_id: locationId.trim() },
            },
            {
              onSuccess: (data) => setGhlMetadataPreview(data),
              onError: (e) =>
                toast.error(`Connected but failed to load GHL metadata: ${errorMessage(e)}`),
            }
          );
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
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
    if (slug === "ghl") {
      if (ghlWebhookMode) {
        if (!ghlConfig.webhook_url?.trim()) {
          toast.error("GHL automation webhook URL is required");
          return;
        }
        const config = normalizeGhlConfig({ ...ghlConfig, create_contact: true });
        create.mutate(
          {
            provider_slug: "ghl",
            name,
            credentials: {},
            config,
          },
          {
            onSuccess: (conn) => {
              setActiveConnection(conn);
              setGhlActive(true);
              setGhlConfig(normalizeGhlConfig({ ...config, ...(conn.config as GHLConfig) }));
              if (conn.inbound_webhook) setInboundWebhook(conn.inbound_webhook);
              toast.success("GoHighLevel connected");
            },
            onError: (e) => toast.error(errorMessage(e)),
          }
        );
        return;
      }
      if (!pitToken.trim()) {
        toast.error("Private Integration Token is required");
        return;
      }
      if (!locationId.trim()) {
        toast.error("Location ID is required");
        return;
      }
      const config = normalizeGhlConfig({
        ...DEFAULT_GHL_CONFIG(locationId.trim()),
        ...ghlConfig,
        location_id: locationId.trim(),
      });
      create.mutate(
        {
          provider_slug: "ghl",
          name,
          credentials: { private_integration_token: pitToken.trim() },
          config,
        },
        {
          onSuccess: (conn) => {
            setActiveConnection(conn);
            setGhlActive(true);
            setGhlConfig(normalizeGhlConfig({ ...config, ...(conn.config as GHLConfig) }));
            if (conn.inbound_webhook) setInboundWebhook(conn.inbound_webhook);
            toast.success("GoHighLevel connected");
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
      return;
    }
    let credentials: Record<string, unknown> = {};
    const config: Record<string, unknown> = {};
    let connectionName = name;
    if (slug === "google_maps") {
      if (isManage) return;
      credentials = { api_key: apiKey };
      connectionName = "Google Maps";
    }
    if (slug === "twilio") {
      credentials = { account_sid: twilioSid.trim(), auth_token: twilioToken.trim() };
    }
    create.mutate(
      { provider_slug: slug, name: connectionName, credentials, config },
      {
        onSuccess: () => {
          toast.success("Connection created");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function saveGhlChanges() {
    if (!activeConnection) return;
    const config = normalizeGhlConfig({
      ...ghlConfig,
      ...(ghlWebhookMode ? {} : { location_id: locationId.trim() }),
      create_contact: true,
    });
    update.mutate(
      {
        id: activeConnection.id,
        credentials:
          !ghlWebhookMode && pitToken.trim()
            ? { private_integration_token: pitToken.trim() }
            : undefined,
        config,
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
          setGhlActive(false);
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
      width={720}
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
        ) : showGhlActive ? (
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
            <Button disabled={update.isPending} onClick={saveGhlChanges}>
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
                (!isSunbase && !isGoogleMaps && !name) ||
                (isGoogleMaps && !apiKey.trim()) ||
                (isTwilio && (!twilioSid.trim() || !twilioToken.trim())) ||
                (isGhl &&
                  !ghlWebhookMode &&
                  !pitToken.trim() &&
                  !showGhlActive) ||
                (isGhl &&
                  !ghlWebhookMode &&
                  !locationId.trim() &&
                  !showGhlActive) ||
                (isGhl &&
                  ghlWebhookMode &&
                  !ghlConfig.webhook_url?.trim() &&
                  !showGhlActive) ||
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

        {showGhlActive && inboundWebhook && <GhlInboundEndpointSection inbound={inboundWebhook} />}

        {isManage && !showSunbaseActive && !showGhlActive && (
          <div className="space-y-2 border-b border-gray-100 pb-4">
            <p className="text-sm font-semibold text-gray-800">Connected</p>
            {existingForSlug.map((c) => (
              <div key={c.id} className="flex items-center justify-between gap-2">
                <button
                  type="button"
                  className="text-left text-sm text-indigo-600 hover:underline"
                  onClick={() => {
                    if (c.provider_slug === "sunbase") loadSunbaseConnection(c);
                    if (c.provider_slug === "ghl") loadGhlConnection(c);
                  }}
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
            {!showSunbaseActive && !showGhlActive && !(isGoogleMaps && isManage) && (
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
                {slug !== "sunbase" && slug !== "google_maps" && (
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

            {(slug === "ghl" || showGhlActive) && (
              <div className="space-y-3">
                <GhlConnectionSettings
                  config={ghlConfig}
                  onChange={setGhlConfig}
                  ghlPipelines={ghlPipelines}
                  ghlCalendars={ghlCalendars}
                  ghlPipelinesLoading={ghlPipelinesBusy}
                  ghlCalendarsLoading={ghlCalendarsBusy}
                  apiAuth={
                    ghlWebhookMode
                      ? undefined
                      : {
                          pitToken,
                          onPitTokenChange: setPitToken,
                          locationId,
                          onLocationIdChange: setLocationId,
                          pitPlaceholder: showGhlActive ? "Leave blank to keep current token" : undefined,
                        }
                  }
                  onTestConnection={runTestGhlConnection}
                  testConnectionDisabled={
                    ghlWebhookMode
                      ? !ghlConfig.webhook_url?.trim() || testConn.isPending
                      : !pitToken.trim() ||
                        !locationId.trim() ||
                        testConn.isPending ||
                        fetchGhlMetadata.isPending
                  }
                  testConnectionPending={
                    testConn.isPending || (!ghlWebhookMode && fetchGhlMetadata.isPending)
                  }
                  testConnectionLoadingLabel={
                    !ghlWebhookMode && fetchGhlMetadata.isPending ? "Loading GHL data…" : "Testing…"
                  }
                />
              </div>
            )}
            {slug === "google_maps" && (
              <>
                <p className="text-sm text-gray-500">
                  Enable Places API (New) and Maps Static API on your Google Cloud key. Restrict the key to your
                  server IP addresses.
                </p>
                <div>
                  <Label>Google Maps API key</Label>
                  <Input type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} />
                </div>
              </>
            )}
            {isTwilio && (
              <>
                <p className="text-sm text-gray-500">
                  Connect your Twilio account to route inbound calls. Credentials are stored encrypted and
                  used to dial buyers and fetch call recordings.
                </p>
                <div>
                  <Label>Account SID</Label>
                  <Input
                    value={twilioSid}
                    onChange={(e) => setTwilioSid(e.target.value)}
                    placeholder="ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
                  />
                </div>
                <div>
                  <Label>Auth token</Label>
                  <Input type="password" value={twilioToken} onChange={(e) => setTwilioToken(e.target.value)} />
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
