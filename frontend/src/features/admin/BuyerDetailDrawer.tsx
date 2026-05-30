import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { cn, formatMoney } from "@/lib/utils";
import { TIMEZONES } from "@/lib/timezones";
import {
  useBuyer,
  useBuyerCollaboration,
  useImpersonateBuyer,
  useRequestCollaboration,
  useUpdateBuyer,
} from "@/features/admin/hooks";
import { useAuthStore } from "@/store/authStore";
import type { CurrentUser } from "@/types";

function collabBadge(status: string) {
  switch (status) {
    case "active":
      return { label: "Collaboration active", className: "bg-green-100 text-green-800" };
    case "pending_buyer":
      return { label: "Awaiting buyer approval", className: "bg-amber-100 text-amber-800" };
    case "pending_publisher":
      return { label: "Awaiting publisher approval", className: "bg-amber-100 text-amber-800" };
    case "revoked":
      return { label: "Revoked", className: "bg-gray-100 text-gray-600" };
    default:
      return { label: "No collaboration", className: "bg-gray-100 text-gray-600" };
  }
}

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
  const { data: collab } = useBuyerCollaboration(buyerId);
  const update = useUpdateBuyer();
  const requestCollab = useRequestCollaboration();
  const impersonate = useImpersonateBuyer();
  const startImpersonation = useAuthStore((s) => s.startImpersonation);
  const navigate = useNavigate();

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
  const collabStatus = collab?.status ?? "none";
  const badge = collabBadge(collabStatus);

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

  function loginAsBuyer() {
    if (!buyer) return;
    impersonate.mutate(buyer.public_id, {
      onSuccess: (res) => {
        const u = res.user as unknown as CurrentUser & { buyer_account_name?: string };
        startImpersonation(res.access, u, u.buyer_account_name ?? buyer.name);
        onClose();
        navigate("/b");
      },
      onError: (e) => toast.error(apiError(e).message),
    });
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
        <div className="mb-5 rounded-lg border border-gray-100 bg-gray-50 p-3">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-sm font-semibold text-gray-800">Collaboration</span>
            <span className={cn("rounded-full px-2 py-0.5 text-xs font-medium", badge.className)}>
              {badge.label}
            </span>
          </div>
          <div className="flex flex-wrap gap-2">
            {collabStatus === "active" && (
              <Button size="sm" disabled={impersonate.isPending} onClick={loginAsBuyer}>
                Login as Buyer
              </Button>
            )}
            {(collabStatus === "none" || collabStatus === "revoked") && (
              <Button
                size="sm"
                variant="secondary"
                disabled={requestCollab.isPending}
                onClick={() =>
                  requestCollab.mutate(buyerId, {
                    onSuccess: () => toast.success("Collaboration request sent"),
                    onError: (e) => toast.error(apiError(e).message),
                  })
                }
              >
                Request collaboration
              </Button>
            )}
          </div>
        </div>

        <div className="flex flex-col gap-2.5">
          <div>
            <div className="mb-2 text-sm font-semibold text-gray-800">Admin</div>
            <div className="space-y-2.5">
              <div>
                <Label>Name</Label>
                <div className="pt-1 text-sm font-medium text-gray-800">
                  {buyer.admin_name || "—"}
                </div>
              </div>
              <div>
                <Label>Email</Label>
                <div className="pt-1 text-sm font-medium text-gray-800">
                  {buyer.admin_email || "—"}
                </div>
              </div>
            </div>
          </div>
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
