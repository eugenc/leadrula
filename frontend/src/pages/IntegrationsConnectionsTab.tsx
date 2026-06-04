import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { SystemIntegrationsCatalog } from "@/features/integrations/SystemIntegrationsCatalog";
import {
  useIntegrationProviders,
  useIntegrationConnections,
  useCreateIntegrationConnection,
  useDeleteIntegrationConnection,
  useOAuthConnect,
} from "@/features/integrations/hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { IntegrationProvider } from "@/types";

export function IntegrationsConnectionsTab() {
  const [params] = useSearchParams();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [preselectedSlug, setPreselectedSlug] = useState<string | undefined>();

  const { data: providers } = useIntegrationProviders();
  const { data: connections, isLoading } = useIntegrationConnections();
  const remove = useDeleteIntegrationConnection();

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
      <PageHeader
        action={<Button onClick={() => openDrawer()}>Add connection</Button>}
      />

      <SystemIntegrationsCatalog
        providers={providers ?? []}
        onConnect={(slug) => openDrawer(slug)}
      />

      <h2 className="mb-3 text-sm font-semibold text-gray-700">Your connections</h2>
      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (connections ?? []).length === 0 ? (
        <EmptyState title="No connections yet. Connect an integration above." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Name</TH>
              <TH>Provider</TH>
              <TH>Status</TH>
              <TH />
            </tr>
          </THead>
          <TBody>
            {(connections ?? []).map((c) => (
              <TR key={c.id}>
                <TD className="font-semibold">{c.name}</TD>
                <TD>{c.provider_name}</TD>
                <TD>{c.status}</TD>
                <TD>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() =>
                      remove.mutate(c.id, { onError: (e) => toast.error(errorMessage(e)) })
                    }
                  >
                    Delete
                  </Button>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      <AddConnectionDrawer
        open={drawerOpen}
        onClose={closeDrawer}
        providers={providers ?? []}
        initialSlug={preselectedSlug}
      />
    </>
  );
}

function AddConnectionDrawer({
  open,
  onClose,
  providers,
  initialSlug,
}: {
  open: boolean;
  onClose: () => void;
  providers: IntegrationProvider[];
  initialSlug?: string;
}) {
  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [secret, setSecret] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [locationId, setLocationId] = useState("");
  const [apiDomain, setApiDomain] = useState("com");

  const create = useCreateIntegrationConnection();
  const oauth = useOAuthConnect();
  const selected = providers.find((p) => p.slug === slug);

  useEffect(() => {
    if (!open) {
      setSlug("");
      setName("");
      setUrl("");
      setSecret("");
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
    if (slug === "webhook") {
      credentials = { url, secret, headers: {} };
    } else if (slug === "ghl") {
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

  return (
    <FormDrawer
      open={open}
      onClose={onClose}
      title="Add connection"
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
        <div>
          <Label>Provider</Label>
          <Select value={slug} onChange={(e) => setSlug(e.target.value)}>
            <option value="">Select…</option>
            {providers
              .filter((p) => p.direction === "outbound" || p.direction === "both")
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
        {slug === "webhook" && (
          <>
            <div>
              <Label>Webhook URL</Label>
              <Input value={url} onChange={(e) => setUrl(e.target.value)} />
            </div>
            <div>
              <Label>HMAC secret (optional)</Label>
              <Input value={secret} onChange={(e) => setSecret(e.target.value)} />
            </div>
          </>
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
          <p className="text-sm text-muted-foreground">
            You will be redirected to {selected.name} to authorize access.
          </p>
        )}
      </div>
    </FormDrawer>
  );
}
