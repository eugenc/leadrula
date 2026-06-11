import { useState } from "react";
import {
  useApiKeys,
  useCreateApiKey,
} from "@/features/admin/hooks";
import { ApiKeyDetailDrawer } from "@/features/admin/ApiKeyDetailDrawer";
import { PageHeader } from "@/components/layout/PageHeader";
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

export function ApiKeysPage() {
  const { data: keys, isLoading } = useApiKeys();
  const create = useCreateApiKey();

  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);
  const [createdKeyId, setCreatedKeyId] = useState<number | null>(null);

  const [selectedKey, setSelectedKey] = useState<ApiKey | null>(null);
  const [detailSecret, setDetailSecret] = useState<string | null>(null);

  function openCreate() {
    setCreateName("");
    setCreatedSecret(null);
    setCreatedKeyId(null);
    setCreateOpen(true);
  }

  function closeCreate() {
    setCreateOpen(false);
    setCreateName("");
    setCreatedSecret(null);
    setCreatedKeyId(null);
  }

  function openDetail(key: ApiKey) {
    setSelectedKey(key);
    setDetailSecret(key.id === createdKeyId ? createdSecret : null);
  }

  function closeDetail() {
    setSelectedKey(null);
    setDetailSecret(null);
  }

  function handleGenerate() {
    const name = createName.trim();
    if (!name) {
      toast.error("Key name is required");
      return;
    }
    create.mutate(name, {
      onSuccess: (res) => {
        setCreatedSecret(res.secret);
        setCreatedKeyId(res.key.id);
        setCreateName("");
        toast.success("API key created");
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  const resolvedSelected =
    selectedKey && keys ? keys.find((k) => k.id === selectedKey.id) ?? selectedKey : selectedKey;

  return (
    <>
      <PageHeader
        action={
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" /> Generate Key
          </Button>
        }
      />

      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (keys ?? []).length === 0 ? (
        <EmptyState title="No API keys yet." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Name</TH>
              <TH>Prefix</TH>
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

      <ApiKeyDetailDrawer
        apiKey={resolvedSelected}
        initialSecret={detailSecret}
        onClose={closeDetail}
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
          <div>
            <Label>Key name</Label>
            <Input
              value={createName}
              onChange={(e) => setCreateName(e.target.value)}
              placeholder="e.g. intake"
              onKeyDown={(e) => {
                if (e.key === "Enter") handleGenerate();
              }}
            />
          </div>
        )}
      </FormDrawer>
    </>
  );
}
