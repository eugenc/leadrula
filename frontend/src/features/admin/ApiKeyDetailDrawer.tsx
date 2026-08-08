import { useEffect, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { IconButton } from "@/components/layout/IconButton";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Badge } from "@/components/ui/misc";
import { Copy, Eye, EyeOff, KeyRound } from "lucide-react";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useUpdateApiKey, useRotateApiKey, useRevokeApiKey, useDeleteApiKey, useRenewApiKey } from "@/features/admin/hooks";
import type { ApiKey } from "@/types";

function copyText(text: string, label: string) {
  navigator.clipboard.writeText(text).then(
    () => toast.success(label),
    () => toast.error("Could not copy to clipboard")
  );
}

export function ApiKeyDetailDrawer({
  apiKey,
  initialSecret,
  onClose,
  onSecretCached,
}: {
  apiKey: ApiKey | null;
  initialSecret?: string | null;
  onClose: () => void;
  onSecretCached?: (keyId: number, secret: string) => void;
}) {
  return (
    <Sheet open={!!apiKey} onClose={onClose}>
      {apiKey && (
        <DrawerContent
          apiKey={apiKey}
          initialSecret={initialSecret}
          onClose={onClose}
          onSecretCached={onSecretCached}
        />
      )}
    </Sheet>
  );
}

function DrawerContent({
  apiKey,
  initialSecret,
  onClose,
  onSecretCached,
}: {
  apiKey: ApiKey;
  initialSecret?: string | null;
  onClose: () => void;
  onSecretCached?: (keyId: number, secret: string) => void;
}) {
  const update = useUpdateApiKey();
  const rotate = useRotateApiKey();
  const revoke = useRevokeApiKey();
  const del = useDeleteApiKey();
  const renew = useRenewApiKey();

  const [name, setName] = useState(apiKey.name);
  const [keyPrefix, setKeyPrefix] = useState(apiKey.key_prefix);
  const [secret, setSecret] = useState<string | null>(initialSecret ?? null);
  const [showSecret, setShowSecret] = useState(false);
  const [revokedAt, setRevokedAt] = useState(apiKey.revoked_at);

  useEffect(() => {
    setName(apiKey.name);
    setKeyPrefix(apiKey.key_prefix);
    setSecret(initialSecret ?? null);
    setShowSecret(false);
    setRevokedAt(apiKey.revoked_at);
  }, [apiKey, initialSecret]);

  const trimmedName = name.trim();
  const active = !revokedAt;
  const unchanged = trimmedName === apiKey.name;
  const invalid = !trimmedName;
  const saving = update.isPending;

  function save() {
    if (unchanged || invalid) return;
    update.mutate(
      { id: apiKey.id, name: trimmedName },
      {
        onSuccess: () => {
          toast.success("Saved");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function handleRotate() {
    rotate.mutate(apiKey.id, {
      onSuccess: (res) => {
        setKeyPrefix(res.key.key_prefix);
        setSecret(res.secret);
        setShowSecret(true);
        onSecretCached?.(apiKey.id, res.secret);
        copyText(res.secret, "New key copied to clipboard");
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  function handleRevoke() {
    revoke.mutate(apiKey.id, {
      onSuccess: () => {
        toast.success("API key revoked");
        onClose();
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  function handleRenew() {
    renew.mutate(apiKey.id, {
      onSuccess: (res) => {
        setKeyPrefix(res.key.key_prefix);
        setSecret(res.secret);
        setRevokedAt(null);
        setShowSecret(true);
        onSecretCached?.(apiKey.id, res.secret);
        copyText(res.secret, "New key copied to clipboard");
        toast.success("API key renewed");
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  function handleDelete() {
    if (!window.confirm("Permanently delete this API key? This cannot be undone.")) return;
    del.mutate(apiKey.id, {
      onSuccess: () => {
        toast.success("API key deleted");
        onClose();
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={apiKey.name}
        subtitle={active ? "Active API key" : "Revoked API key"}
        onClose={onClose}
      />

      <DrawerBody>
        <div className="flex flex-col gap-2.5">
          <div>
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <Label>API key</Label>
            {secret ? (
              <div className="flex items-center gap-2 pt-1">
                <code className="flex-1 break-all rounded-md bg-gray-50 px-3 py-2 font-mono text-xs">
                  {showSecret ? secret : "••••••••••••••••••••••••••••••••"}
                </code>
                <IconButton
                  aria-label={showSecret ? "Hide key" : "Show key"}
                  onClick={() => setShowSecret((v) => !v)}
                >
                  {showSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </IconButton>
                <IconButton
                  aria-label="Copy API key"
                  onClick={() => copyText(secret, "API key copied")}
                >
                  <Copy className="h-4 w-4" />
                </IconButton>
              </div>
            ) : (
              <div className="space-y-1 pt-1">
                <code className="block rounded-md bg-gray-50 px-3 py-2 font-mono text-xs text-gray-500">
                  {keyPrefix}…
                </code>
                <p className="text-xs text-gray-500">
                  Full key is only available after creation or rotation.
                </p>
              </div>
            )}
          </div>
          <div>
            <Label>Status</Label>
            <div className="pt-1">
              <Badge variant={active ? "distributed" : "closed"}>
                {active ? "active" : "revoked"}
              </Badge>
            </div>
          </div>
          <div>
            <Label>Last used</Label>
            <p className="pt-1 text-sm text-gray-600">
              {apiKey.last_used_at
                ? format(new Date(apiKey.last_used_at), "MMM d, yyyy h:mma")
                : "Never"}
            </p>
          </div>
        </div>
      </DrawerBody>

      <DrawerFooter className="flex flex-col gap-2">
        <Button disabled={unchanged || invalid || saving} onClick={save}>
          Save
        </Button>
        {active && (
          <Button variant="secondary" disabled={rotate.isPending} onClick={handleRotate}>
            <KeyRound className="mr-2 h-4 w-4" />
            Rotate key
          </Button>
        )}
        {!active && (
          <Button variant="secondary" disabled={renew.isPending} onClick={handleRenew}>
            <KeyRound className="mr-2 h-4 w-4" />
            Renew key
          </Button>
        )}
        {active && (
          <Button variant="danger" disabled={revoke.isPending} onClick={handleRevoke}>
            Revoke key
          </Button>
        )}
        <Button variant="danger" disabled={del.isPending} onClick={handleDelete}>
          Delete key
        </Button>
      </DrawerFooter>
    </div>
  );
}
