import { useEffect, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { cn, formatMoney } from "@/lib/utils";
import { TIMEZONES } from "@/lib/timezones";
import { useBuyer, useUpdateBuyer } from "@/features/admin/hooks";

export function BuyerDetailDrawer({
  buyerId,
  leadCount,
  onClose,
}: {
  buyerId: number | null;
  leadCount: number;
  onClose: () => void;
}) {
  return (
    <Sheet open={!!buyerId} onClose={onClose} width={520}>
      {buyerId && <DrawerContent buyerId={buyerId} leadCount={leadCount} onClose={onClose} />}
    </Sheet>
  );
}

function DrawerContent({
  buyerId,
  leadCount,
  onClose,
}: {
  buyerId: number;
  leadCount: number;
  onClose: () => void;
}) {
  const { data: buyer, isLoading } = useBuyer(buyerId);
  const update = useUpdateBuyer();

  const [name, setName] = useState("");
  const [website, setWebsite] = useState("");
  const [timezone, setTimezone] = useState("America/Toronto");

  useEffect(() => {
    if (!buyer) return;
    setName(buyer.name);
    setWebsite(buyer.website);
    setTimezone(buyer.timezone);
  }, [buyer]);

  const trimmedName = name.trim();
  const trimmedWebsite = website.trim();
  const unchanged =
    buyer != null &&
    trimmedName === buyer.name &&
    trimmedWebsite === buyer.website &&
    timezone === buyer.timezone;
  const invalid = !trimmedName;
  const saving = update.isPending;

  function save() {
    if (!buyer) return;
    const body: Record<string, string> = {};
    if (trimmedName !== buyer.name) body.name = trimmedName;
    if (trimmedWebsite !== buyer.website) body.website = trimmedWebsite;
    if (timezone !== buyer.timezone) body.timezone = timezone;
    if (Object.keys(body).length === 0) return;

    update.mutate(
      { id: buyerId, body },
      {
        onSuccess: () => {
          toast.success("Saved");
          onClose();
        },
        onError: (e) => toast.error(apiError(e).message),
      }
    );
  }

  if (isLoading || !buyer) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={buyer.name}
        subtitle={`${leadCount} leads · ${formatMoney(buyer.balance)}`}
        onClose={onClose}
      />

      <DrawerBody>
        <div className="flex flex-col gap-2.5">
          <div>
            <Label>Company Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <Label>Website</Label>
            <Input
              type="url"
              placeholder="https://example.com"
              value={website}
              onChange={(e) => setWebsite(e.target.value)}
            />
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
            <Label>Balance</Label>
            <div className={cn("pt-1 text-sm font-medium text-gray-800", buyer.balance < 0 && "text-danger")}>
              {formatMoney(buyer.balance)}
            </div>
          </div>
          <div>
            <Label>Leads</Label>
            <div className="pt-1 text-sm font-medium text-gray-800">{leadCount}</div>
          </div>
        </div>
      </DrawerBody>

      <DrawerFooter>
        <Button disabled={unchanged || invalid || saving} onClick={save}>
          Save
        </Button>
      </DrawerFooter>
    </div>
  );
}
