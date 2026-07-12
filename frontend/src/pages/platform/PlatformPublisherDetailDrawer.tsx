import { useEffect, useState } from "react";
import { format } from "date-fns";
import { ArrowRightLeft } from "lucide-react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { MessageButton } from "@/features/messaging/MessageButton";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { TIMEZONES } from "@/lib/timezones";
import {
  useRemovePlatformPublisher,
  useSwitchAccount,
  useUpdatePublisher,
} from "@/features/auth/switchHooks";
import { PlatformAccountStatusCell } from "@/pages/platform/PlatformAccountStatusCell";
import { RemovePlatformAccountDialog } from "@/pages/platform/RemovePlatformAccountDialog";
import type { AccountOperationalStatus, PlatformAccount } from "@/types";

export function PlatformPublisherDetailDrawer({
  publisher,
  onClose,
}: {
  publisher: PlatformAccount | null;
  onClose: () => void;
}) {
  return (
    <Sheet open={!!publisher} onClose={onClose} width={520}>
      {publisher && <DrawerContent publisher={publisher} onClose={onClose} />}
    </Sheet>
  );
}

function DrawerContent({
  publisher,
  onClose,
}: {
  publisher: PlatformAccount;
  onClose: () => void;
}) {
  const update = useUpdatePublisher();
  const remove = useRemovePlatformPublisher();
  const switchAccount = useSwitchAccount();
  const [removeOpen, setRemoveOpen] = useState(false);
  const [name, setName] = useState(publisher.name);
  const [timezone, setTimezone] = useState(publisher.timezone);
  const [status, setStatus] = useState<AccountOperationalStatus>(
    publisher.operational_status ?? "active"
  );

  useEffect(() => {
    setName(publisher.name);
    setTimezone(publisher.timezone);
    setStatus(publisher.operational_status ?? "active");
  }, [publisher]);

  const trimmedName = name.trim();
  const unchanged =
    trimmedName === publisher.name &&
    timezone === publisher.timezone &&
    status === (publisher.operational_status ?? "active");
  const invalid = !trimmedName;
  const saving = update.isPending || remove.isPending;

  function save() {
    const body: {
      name?: string;
      timezone?: string;
      operational_status?: AccountOperationalStatus;
    } = {};
    if (trimmedName !== publisher.name) body.name = trimmedName;
    if (timezone !== publisher.timezone) body.timezone = timezone;
    if (status !== (publisher.operational_status ?? "active")) {
      body.operational_status = status;
    }
    if (Object.keys(body).length === 0) return;

    update.mutate(
      { id: publisher.id, body },
      {
        onSuccess: () => {
          toast.success("Publisher updated");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader title={publisher.name} subtitle={publisher.handler_id} onClose={onClose} />

      <DrawerBody>
        <div className="mb-4">
          <MessageButton accountId={publisher.id} />
        </div>
        <div className="flex flex-col gap-2.5">
          <div>
            <Label>Publisher name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <Label>Timezone</Label>
            <Select value={timezone} onChange={(e) => setTimezone(e.target.value)}>
              {TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Status</Label>
            <PlatformAccountStatusCell value={status} disabled={saving} onChange={setStatus} />
          </div>
          <div>
            <Label>Handler ID</Label>
            <div className="pt-1 font-mono text-sm text-gray-600">{publisher.handler_id}</div>
          </div>
          <div>
            <Label>Created</Label>
            <div className="pt-1 text-sm text-gray-600">
              {publisher.created_at
                ? format(new Date(publisher.created_at), "MMM d, yyyy")
                : "—"}
            </div>
          </div>
        </div>
      </DrawerBody>

      <DrawerFooter>
        <div className="flex items-center justify-between gap-2">
          <div className="flex gap-2">
            <Button
              variant="secondary"
              disabled={switchAccount.isPending || status === "suspended"}
              title={status === "suspended" ? "Account suspended" : undefined}
              onClick={() => switchAccount.mutate(publisher.id)}
            >
              <ArrowRightLeft className="h-3.5 w-3.5" /> Open
            </Button>
            <Button variant="danger" disabled={saving} onClick={() => setRemoveOpen(true)}>
              Remove
            </Button>
          </div>
          <div className="flex gap-2">
            <Button variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button disabled={invalid || unchanged || saving} onClick={save}>
              Save
            </Button>
          </div>
        </div>
      </DrawerFooter>

      <RemovePlatformAccountDialog
        open={removeOpen}
        accountName={publisher.name}
        accountType="publisher"
        loading={remove.isPending}
        onClose={() => setRemoveOpen(false)}
        onConfirm={() =>
          remove.mutate(publisher.id, {
            onSuccess: () => {
              toast.success("Publisher removed");
              setRemoveOpen(false);
              onClose();
            },
            onError: (e) => toast.error(errorMessage(e)),
          })
        }
      />
    </div>
  );
}
