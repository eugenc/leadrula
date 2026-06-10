import { useState } from "react";
import { useApiKeys, useCreateApiKey, useRevokeApiKey } from "@/features/admin/hooks";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, Badge, Spinner, EmptyState } from "@/components/ui/misc";
import { Plus, Copy } from "lucide-react";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";

export function ApiKeysPage() {
  const { data: keys, isLoading } = useApiKeys();
  const create = useCreateApiKey();
  const revoke = useRevokeApiKey();
  const [name, setName] = useState("");
  const [secret, setSecret] = useState<string | null>(null);

  return (
    <>
        {secret && (
          <Card className="mb-4 border-jade-200 bg-jade-50 p-4">
            <div className="mb-1 text-sm font-semibold text-jade-700">
              Copy your key now — it won't be shown again.
            </div>
            <div className="flex items-center gap-2">
              <code className="flex-1 break-all rounded-md bg-surface-card px-3 py-2 font-mono text-xs">
                {secret}
              </code>
              <Button
                size="icon"
                variant="secondary"
                onClick={() => {
                  navigator.clipboard.writeText(secret);
                  toast.success("Copied");
                }}
              >
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </Card>
        )}

        <div className="mb-4 flex gap-2">
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Key name" className="w-64" />
          <Button
            onClick={() =>
              name &&
              create.mutate(name, {
                onSuccess: (res) => {
                  setSecret(res.secret);
                  setName("");
                },
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
          >
            <Plus className="h-4 w-4" /> Generate Key
          </Button>
        </div>

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
                <TH className="min-w-0 w-12" />
              </tr>
            </THead>
            <TBody>
              {(keys ?? []).map((k) => (
                <TR key={k.id}>
                  <TD className="font-medium text-gray-800">{k.name}</TD>
                  <TD className="font-mono text-xs">{k.key_prefix}…</TD>
                  <TD>{k.last_used_at ? format(new Date(k.last_used_at), "MMM d, h:mma") : "never"}</TD>
                  <TD>
                    <Badge variant={k.revoked_at ? "closed" : "distributed"}>
                      {k.revoked_at ? "revoked" : "active"}
                    </Badge>
                  </TD>
                  <TD>
                    {!k.revoked_at && (
                      <Button size="sm" variant="secondary" onClick={() => revoke.mutate(k.id)}>
                        Revoke
                      </Button>
                    )}
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}
    </>
  );
}
