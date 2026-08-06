import { useEffect, useState } from "react";
import { format } from "date-fns";
import { ArrowRightLeft } from "lucide-react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { MessageButton } from "@/features/messaging/MessageButton";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { cn, formatMoney } from "@/lib/utils";
import { TIMEZONES } from "@/lib/timezones";
import {
  useCreditPlatformBuyer,
  usePlatformBuyerBalance,
  useRemovePlatformBuyer,
  useSwitchAccount,
  useUpdatePlatformBuyer,
} from "@/features/auth/switchHooks";
import { PlatformAccountStatusCell } from "@/pages/platform/PlatformAccountStatusCell";
import { CreditPlatformBuyerDialog } from "@/pages/platform/CreditPlatformBuyerDialog";
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
  const credit = useCreditPlatformBuyer();
  const { data: balanceData, isLoading: balanceLoading } = usePlatformBuyerBalance(buyer.id);
  const [removeOpen, setRemoveOpen] = useState(false);
  const [creditOpen, setCreditOpen] = useState(false);
  const [name, setName] = useState(buyer.name);
  const [timezone, setTimezone] = useState(buyer.timezone);
  const [status, setStatus] = useState<AccountOperationalStatus>(buyer.operational_status ?? "active");
  const [creditAmount, setCreditAmount] = useState("");
  const [creditNote, setCreditNote] = useState("");

  useEffect(() => {
    setName(buyer.name);
    setTimezone(buyer.timezone);
    setStatus(buyer.operational_status ?? "active");
    setCreditAmount("");
    setCreditNote("");
    setCreditOpen(false);
  }, [buyer]);

  const trimmedName = name.trim();
  const unchanged =
    trimmedName === buyer.name &&
    timezone === buyer.timezone &&
    status === (buyer.operational_status ?? "active");
  const invalid = !trimmedName;
  const saving = update.isPending || remove.isPending;
  const parsedCreditAmount = parseFloat(creditAmount);
  const creditAmountValid = !Number.isNaN(parsedCreditAmount) && parsedCreditAmount > 0;
  const balance = balanceData?.balance ?? 0;

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

  function confirmCredit() {
    if (!creditAmountValid) return;
    credit.mutate(
      { id: buyer.id, amount: parsedCreditAmount, note: creditNote },
      {
        onSuccess: (res) => {
          toast.success(`Added ${formatMoney(res.amount)} — new balance ${formatMoney(res.balance)}`);
          setCreditOpen(false);
          setCreditAmount("");
          setCreditNote("");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader title={buyer.name} subtitle={buyer.handler_id} onClose={onClose} />

      <DrawerBody>
        <div className="mb-4">
          <MessageButton accountId={buyer.id} />
        </div>
        <div className="flex flex-col gap-2.5">
          <div>
            <Label>Balance</Label>
            <div
              className={cn(
                "pt-1 text-sm font-medium text-gray-800",
                balance < 0 && "text-danger"
              )}
            >
              {balanceLoading ? <Spinner className="h-4 w-4" /> : formatMoney(balance)}
            </div>
          </div>
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

          <div className="mt-2 border-t border-gray-100 pt-4">
            <div className="mb-2 text-sm font-semibold text-gray-800">Add funds</div>
            <div className="space-y-2.5">
              <div>
                <Label>Amount (USD)</Label>
                <Input
                  type="number"
                  min={0.01}
                  step="0.01"
                  value={creditAmount}
                  onChange={(e) => setCreditAmount(e.target.value)}
                  disabled={credit.isPending}
                />
              </div>
              <div>
                <Label>Note (optional)</Label>
                <Input
                  value={creditNote}
                  onChange={(e) => setCreditNote(e.target.value)}
                  placeholder="Reason for credit"
                  disabled={credit.isPending}
                />
              </div>
              <Button
                disabled={!creditAmountValid || credit.isPending}
                onClick={() => setCreditOpen(true)}
              >
                Add funds
              </Button>
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

      <CreditPlatformBuyerDialog
        open={creditOpen}
        buyerName={buyer.name}
        amount={parsedCreditAmount}
        note={creditNote}
        loading={credit.isPending}
        onClose={() => setCreditOpen(false)}
        onConfirm={confirmCredit}
      />

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
