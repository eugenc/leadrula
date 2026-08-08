import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import {
  useApiKeys,
  useCreateApiKey,
} from "@/features/admin/hooks";
import { ApiKeyDetailDrawer } from "@/features/admin/ApiKeyDetailDrawer";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Badge, Spinner, EmptyState } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { Plus, Copy } from "lucide-react";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { ApiKey } from "@/types";

function copyText(text: string, label: string) {
  navigator.clipboard.writeText(text).then(
    () => toast.success(label),
    () => toast.error("Could not copy to clipboard")
  );
}

const API_KEY_SCOPES = [
  { id: "leads:read", label: "Read leads" },
  { id: "leads:write", label: "Write leads" },
  { id: "appointments:read", label: "Read appointments & calendars" },
  { id: "appointments:write", label: "Book appointments" },
] as const;

const DEFAULT_API_KEY_SCOPES = ["leads:read", "leads:write"];

export function ApiKeysPage() {
  const { pathname } = useLocation();
  const prefix = pathname.startsWith("/p/") ? "/p" : "/b";
  const { data: keys, isLoading } = useApiKeys();
  const create = useCreateApiKey();

  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createScopes, setCreateScopes] = useState<string[]>([...DEFAULT_API_KEY_SCOPES]);
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);
  const [createdKeyId, setCreatedKeyId] = useState<number | null>(null);

  const [selectedKey, setSelectedKey] = useState<ApiKey | null>(null);
  const [detailSecret, setDetailSecret] = useState<string | null>(null);
  const [secretByKeyId, setSecretByKeyId] = useState<Record<number, string>>({});

  function openCreate() {
    setCreateName("");
    setCreateScopes([...DEFAULT_API_KEY_SCOPES]);
    setCreatedSecret(null);
    setCreatedKeyId(null);
    setCreateOpen(true);
  }

  function closeCreate() {
    setCreateOpen(false);
    setCreateName("");
    setCreateScopes([...DEFAULT_API_KEY_SCOPES]);
    setCreatedSecret(null);
    setCreatedKeyId(null);
  }

  function openDetail(key: ApiKey) {
    setSelectedKey(key);
    setDetailSecret(
      secretByKeyId[key.id] ?? (key.id === createdKeyId ? createdSecret : null)
    );
  }

  function closeDetail() {
    setSelectedKey(null);
    setDetailSecret(null);
  }

  function toggleScope(scope: string) {
    setCreateScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
    );
  }

  function handleGenerate() {
    const name = createName.trim();
    if (!name) {
      toast.error("Key name is required");
      return;
    }
    if (createScopes.length === 0) {
      toast.error("Select at least one scope");
      return;
    }
    create.mutate(
      { name, scopes: createScopes },
      {
        onSuccess: (res) => {
          setCreatedSecret(res.secret);
          setCreatedKeyId(res.key.id);
          setSecretByKeyId((prev) => ({ ...prev, [res.key.id]: res.secret }));
          setCreateName("");
          toast.success("API key created");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const resolvedSelected =
    selectedKey && keys ? keys.find((k) => k.id === selectedKey.id) ?? selectedKey : selectedKey;

  return (
    <>
      <PageHeader
        className="px-0 pt-0"
        action={
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" /> Generate Key
          </Button>
        }
      />

      <p className="mb-4 text-sm text-gray-500">
        <Link to={`${prefix}/api-docs`} className="text-jade-600 hover:underline">
          View API documentation →
        </Link>
      </p>

      <PageBody className="px-0 pt-0 pb-0">
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (keys ?? []).length === 0 ? (
          <EmptyState title="No API keys yet." />
        ) : (
          <Table>
          <THead>
            <tr>
              <TH>Name</TH>
              <TH>API key</TH>
              <TH>Last used</TH>
              <TH>Status</TH>
            </tr>
          </THead>
          <TBody>
            {(keys ?? []).map((k) => (
              <TR key={k.id} onClick={() => openDetail(k)}>
                <TD className="font-medium text-gray-800">{k.name}</TD>
                <TD className="font-mono text-xs">{k.key_prefix}…</TD>
                <TD>{k.last_used_at ? format(new Date(k.last_used_at), "MMM d, h:mma") : "never"}</TD>
                <TD>
                  <Badge variant={k.revoked_at ? "closed" : "distributed"}>
                    {k.revoked_at ? "revoked" : "active"}
                  </Badge>
                </TD>
              </TR>
            ))}
          </TBody>
          </Table>
        )}
      </PageBody>

      <ApiKeyDetailDrawer
        apiKey={resolvedSelected}
        initialSecret={
          resolvedSelected
            ? detailSecret ?? secretByKeyId[resolvedSelected.id] ?? null
            : null
        }
        onClose={closeDetail}
        onSecretCached={(keyId, secret) => {
          setSecretByKeyId((prev) => ({ ...prev, [keyId]: secret }));
          setDetailSecret(secret);
        }}
      />

      <FormDrawer
        open={createOpen}
        onClose={closeCreate}
        title={createdSecret ? "Copy your API key" : "Generate API key"}
        subtitle={
          createdSecret
            ? "This is the only time the full key will be shown."
            : "Give the key a name so you can identify it later."
        }
        footer={
          createdSecret ? (
            <Button onClick={closeCreate}>Done</Button>
          ) : (
            <>
              <Button variant="secondary" onClick={closeCreate}>
                Cancel
              </Button>
              <Button disabled={!createName.trim() || create.isPending} onClick={handleGenerate}>
                {create.isPending ? "Generating…" : "Generate Key"}
              </Button>
            </>
          )
        }
      >
        {createdSecret ? (
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <code className="flex-1 break-all rounded-md bg-gray-50 px-3 py-2 font-mono text-xs">
                {createdSecret}
              </code>
              <IconButton
                aria-label="Copy key"
                onClick={() => copyText(createdSecret, "Key copied")}
              >
                <Copy className="h-4 w-4" />
              </IconButton>
            </div>
            <p className="text-xs text-gray-500">
              Store this key securely. You can rotate it later from the key detail panel.
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            <div>
              <Label>Key name</Label>
              <Input
                value={createName}
                onChange={(e) => setCreateName(e.target.value)}
                placeholder="e.g. voiceuni"
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleGenerate();
                }}
              />
            </div>
            <div>
              <Label>Scopes</Label>
              <div className="mt-2 space-y-2">
                {API_KEY_SCOPES.map((scope) => (
                  <label key={scope.id} className="flex items-center gap-2 text-sm text-gray-700">
                    <input
                      type="checkbox"
                      className="rounded"
                      checked={createScopes.includes(scope.id)}
                      onChange={() => toggleScope(scope.id)}
                    />
                    <code className="text-xs">{scope.id}</code>
                    <span className="text-gray-500">— {scope.label}</span>
                  </label>
                ))}
              </div>
            </div>
          </div>
        )}
      </FormDrawer>
    </>
  );
}
