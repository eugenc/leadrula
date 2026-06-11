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
import { useUpdateApiKey, useRotateApiKey, useRevokeApiKey } from "@/features/admin/hooks";
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
}: {
  apiKey: ApiKey | null;
  initialSecret?: string | null;
  onClose: () => void;
}) {
  return (
    <Sheet open={!!apiKey} onClose={onClose}>
      {apiKey && (
        <DrawerContent apiKey={apiKey} initialSecret={initialSecret} onClose={onClose} />
      )}
    </Sheet>
  );
}

function DrawerContent({
  apiKey,
  initialSecret,
  onClose,
}: {
  apiKey: ApiKey;
  initialSecret?: string | null;
  onClose: () => void;
}) {
  const update = useUpdateApiKey();
  const rotate = useRotateApiKey();
  const revoke = useRevokeApiKey();

  const [name, setName] = useState(apiKey.name);
  const [keyPrefix, setKeyPrefix] = useState(apiKey.key_prefix);
  const [secret, setSecret] = useState<string | null>(initialSecret ?? null);
  const [showSecret, setShowSecret] = useState(false);

  useEffect(() => {
    setName(apiKey.name);
    setKeyPrefix(apiKey.key_prefix);
    setSecret(initialSecret ?? null);
    setShowSecret(false);
  }, [apiKey, initialSecret]);

  const trimmedName = name.trim();
  const active = !apiKey.revoked_at;
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
        setShowSecret(false);
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
            <Label>Prefix</Label>
            <div className="flex items-center gap-2 pt-1">
              <code className="flex-1 rounded-md bg-gray-50 px-3 py-2 font-mono text-xs">
                {keyPrefix}…
              </code>
              <IconButton
                aria-label="Copy prefix"
                onClick={() => copyText(keyPrefix, "Prefix copied")}
              >
                <Copy className="h-4 w-4" />
              </IconButton>
            </div>
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
          {secret && (
            <div>
              <Label>Key</Label>
              <p className="mb-1 text-xs text-jade-700">
                Copy now — it will not be shown again after you close this panel.
              </p>
              <div className="flex items-center gap-2">
                <code className="flex-1 break-all rounded-md bg-gray-50 px-3 py-2 font-mono text-xs">
                  {showSecret ? secret : "••••••••••••••••••••••••••••••••"}
                </code>
                <IconButton
                  aria-label={showSecret ? "Hide key" : "Show key"}
                  onClick={() => setShowSecret((v) => !v)}
                >
                  {showSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </IconButton>
                <IconButton aria-label="Copy key" onClick={() => copyText(secret, "Key copied")}>
                  <Copy className="h-4 w-4" />
                </IconButton>
              </div>
            </div>
          )}
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
        {active && (
          <Button variant="danger" disabled={revoke.isPending} onClick={handleRevoke}>
            Revoke key
          </Button>
        )}
      </DrawerFooter>
    </div>
  );
}
