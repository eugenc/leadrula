import { useEffect, useState } from "react";
import { format } from "date-fns";
import { ArrowRightLeft } from "lucide-react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { TIMEZONES } from "@/lib/timezones";
import {
  useRemovePlatformBuyer,
  useSwitchAccount,
  useUpdatePlatformBuyer,
} from "@/features/auth/switchHooks";
import { PlatformAccountStatusCell } from "@/pages/platform/PlatformAccountStatusCell";
import { RemovePlatformAccountDialog } from "@/pages/platform/RemovePlatformAccountDialog";
import type { AccountOperationalStatus, PlatformAccount } from "@/types";

export function PlatformBuyerDetailDrawer({
  buyer,
  onClose,
}: {
  buyer: PlatformAccount | null;
  onClose: () => void;
}) {
  return (
    <Sheet open={!!buyer} onClose={onClose} width={520}>
      {buyer && <DrawerContent buyer={buyer} onClose={onClose} />}
    </Sheet>
  );
}

function DrawerContent({ buyer, onClose }: { buyer: PlatformAccount; onClose: () => void }) {
  const update = useUpdatePlatformBuyer();
  const remove = useRemovePlatformBuyer();
  const switchAccount = useSwitchAccount();
  const [removeOpen, setRemoveOpen] = useState(false);
  const [name, setName] = useState(buyer.name);
  const [timezone, setTimezone] = useState(buyer.timezone);
  const [status, setStatus] = useState<AccountOperationalStatus>(buyer.operational_status ?? "active");

  useEffect(() => {
    setName(buyer.name);
    setTimezone(buyer.timezone);
    setStatus(buyer.operational_status ?? "active");
  }, [buyer]);

  const trimmedName = name.trim();
  const unchanged =
    trimmedName === buyer.name &&
    timezone === buyer.timezone &&
    status === (buyer.operational_status ?? "active");
  const invalid = !trimmedName;
  const saving = update.isPending || remove.isPending;

  function save() {
    const body: {
      name?: string;
      timezone?: string;
      operational_status?: AccountOperationalStatus;
    } = {};
    if (trimmedName !== buyer.name) body.name = trimmedName;
    if (timezone !== buyer.timezone) body.timezone = timezone;
    if (status !== (buyer.operational_status ?? "active")) {
      body.operational_status = status;
    }
    if (Object.keys(body).length === 0) return;

    update.mutate(
      { id: buyer.id, body },
      {
        onSuccess: () => {
          toast.success("Buyer updated");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader title={buyer.name} subtitle={buyer.handler_id} onClose={onClose} />

      <DrawerBody>
        <div className="flex flex-col gap-2.5">
          <div>
            <Label>Company name</Label>
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
            <div className="pt-1 font-mono text-sm text-gray-600">{buyer.handler_id}</div>
          </div>
          <div>
            <Label>Created</Label>
            <div className="pt-1 text-sm text-gray-600">
              {buyer.created_at ? format(new Date(buyer.created_at), "MMM d, yyyy") : "—"}
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
              onClick={() => switchAccount.mutate(buyer.id)}
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
        accountName={buyer.name}
        accountType="buyer"
        loading={remove.isPending}
        onClose={() => setRemoveOpen(false)}
        onConfirm={() =>
          remove.mutate(buyer.id, {
            onSuccess: () => {
              toast.success("Buyer removed");
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
