import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Dialog } from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useTwilioPhoneNumbers,
  useTwilioAvailableNumbers,
  useTwilioPricing,
  usePurchaseTwilioPhoneNumber,
  useReleaseTwilioPhoneNumber,
} from "@/features/integrations/hooks";
import type { TwilioAvailablePhoneNumber } from "@/types";

const TOLL_FREE_PREFIXES = ["800", "888", "877", "866", "855", "844", "833"];

function numberTypeLabel(type?: string): string {
  return type === "tollfree" ? "Toll-free" : "Local";
}

function formatMonthlyPrice(price?: number): string | null {
  if (price == null || Number.isNaN(price)) return null;
  return `$${price.toFixed(2)}/mo`;
}

export function TwilioPhoneNumbersSection({ connectionId }: { connectionId: number }) {
  const { data: owned, isLoading: ownedLoading, isError: ownedError } = useTwilioPhoneNumbers(connectionId);
  const purchase = usePurchaseTwilioPhoneNumber();
  const release = useReleaseTwilioPhoneNumber();

  const [buyTab, setBuyTab] = useState<"local" | "tollfree">("local");
  const [areaCode, setAreaCode] = useState("");
  const [prefix, setPrefix] = useState("800");
  const [searchFilters, setSearchFilters] = useState<{
    type: "local" | "tollfree";
    area_code?: string;
    prefix?: string;
  } | null>(null);

  const { data: available, isFetching: searchLoading, isError: searchError } = useTwilioAvailableNumbers(
    connectionId,
    searchFilters
  );

  const [confirmTarget, setConfirmTarget] = useState<TwilioAvailablePhoneNumber | null>(null);
  const { data: pricing } = useTwilioPricing(connectionId, confirmTarget ? buyTab : null);

  const [releaseTarget, setReleaseTarget] = useState<{ sid: string; phone: string } | null>(null);

  function runSearch() {
    if (buyTab === "local") {
      if (areaCode.trim().length !== 3) {
        toast.error("Enter a 3-digit area code");
        return;
      }
      setSearchFilters({ type: "local", area_code: areaCode.trim() });
      return;
    }
    setSearchFilters({ type: "tollfree", prefix });
  }

  function confirmPurchase() {
    if (!confirmTarget) return;
    purchase.mutate(
      { connectionId, phone_number: confirmTarget.phone_number },
      {
        onSuccess: () => {
          toast.success(`Purchased ${confirmTarget.phone_number}`);
          setConfirmTarget(null);
          setSearchFilters(null);
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function confirmRelease() {
    if (!releaseTarget) return;
    release.mutate(
      { connectionId, sid: releaseTarget.sid },
      {
        onSuccess: () => {
          toast.success(`Released ${releaseTarget.phone}`);
          setReleaseTarget(null);
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const priceLabel = formatMonthlyPrice(pricing?.monthly_price);

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-sm font-semibold text-gray-800">Your numbers</h3>
        <p className="mt-0.5 text-xs text-gray-500">Voice-capable numbers on this Twilio account.</p>
        {ownedLoading && (
          <div className="mt-3 flex items-center gap-2 text-sm text-gray-500">
            <Spinner /> Loading…
          </div>
        )}
        {ownedError && <p className="mt-2 text-sm text-red-600">Failed to load numbers.</p>}
        {!ownedLoading && !ownedError && (owned?.length ?? 0) === 0 && (
          <p className="mt-2 text-sm text-gray-500">No numbers yet. Buy one below.</p>
        )}
        {(owned?.length ?? 0) > 0 && (
          <div className="mt-2 overflow-hidden rounded-md border border-gray-200">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 text-left text-xs text-gray-500">
                <tr>
                  <th className="px-3 py-2 font-medium">Number</th>
                  <th className="px-3 py-2 font-medium">Type</th>
                  <th className="px-3 py-2 font-medium">Status</th>
                  <th className="px-3 py-2 font-medium" />
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {owned!.map((n) => (
                  <tr key={n.sid}>
                    <td className="px-3 py-2 font-mono text-gray-800">{n.phone_number}</td>
                    <td className="px-3 py-2 text-gray-600">{numberTypeLabel(n.number_type)}</td>
                    <td className="px-3 py-2 text-gray-600">
                      {n.in_use_by_source ? (
                        <span className="text-amber-800">
                          In use{n.in_use_active ? "" : " (inactive)"}: {n.in_use_by_source}
                        </span>
                      ) : (
                        <span className="text-gray-400">Available</span>
                      )}
                    </td>
                    <td className="px-3 py-2 text-right">
                      <Button
                        variant="secondary"
                        size="sm"
                        disabled={n.in_use_active || release.isPending}
                        title={
                          n.in_use_active
                            ? `Used by active call source ${n.in_use_by_source}`
                            : "Release number from Twilio"
                        }
                        onClick={() => setReleaseTarget({ sid: n.sid, phone: n.phone_number })}
                      >
                        Release
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="border-t border-gray-100 pt-4">
        <h3 className="text-sm font-semibold text-gray-800">Buy a number</h3>
        <p className="mt-0.5 text-xs text-gray-500">Charges your Twilio account. US numbers only.</p>

        <div className="mt-3 flex gap-2">
          <Button
            variant={buyTab === "local" ? "primary" : "secondary"}
            size="sm"
            onClick={() => {
              setBuyTab("local");
              setSearchFilters(null);
            }}
          >
            Local
          </Button>
          <Button
            variant={buyTab === "tollfree" ? "primary" : "secondary"}
            size="sm"
            onClick={() => {
              setBuyTab("tollfree");
              setSearchFilters(null);
            }}
          >
            Toll-free
          </Button>
        </div>

        <div className="mt-3 flex flex-wrap items-end gap-2">
          {buyTab === "local" ? (
            <div>
              <Label>Area code</Label>
              <Input
                value={areaCode}
                onChange={(e) => setAreaCode(e.target.value.replace(/\D/g, "").slice(0, 3))}
                placeholder="415"
                className="w-24"
              />
            </div>
          ) : (
            <div>
              <Label>Prefix</Label>
              <Select value={prefix} onChange={(e) => setPrefix(e.target.value)} className="w-28">
                {TOLL_FREE_PREFIXES.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </Select>
            </div>
          )}
          <Button
            variant="secondary"
            disabled={buyTab === "local" && areaCode.length !== 3}
            onClick={runSearch}
          >
            Search
          </Button>
        </div>

        {searchLoading && (
          <div className="mt-3 flex items-center gap-2 text-sm text-gray-500">
            <Spinner /> Searching…
          </div>
        )}
        {searchError && <p className="mt-2 text-sm text-red-600">Search failed.</p>}
        {searchFilters && !searchLoading && !searchError && (available?.length ?? 0) === 0 && (
          <p className="mt-2 text-sm text-gray-500">No numbers found. Try another area code or prefix.</p>
        )}
        {(available?.length ?? 0) > 0 && (
          <div className="mt-2 max-h-48 overflow-auto rounded-md border border-gray-200">
            {available!.map((n) => (
              <div
                key={n.phone_number}
                className="flex items-center justify-between gap-2 border-b border-gray-100 px-3 py-2 last:border-0"
              >
                <div className="min-w-0">
                  <p className="font-mono text-sm text-gray-800">{n.phone_number}</p>
                  <p className="truncate text-xs text-gray-500">
                    {[n.locality, n.region].filter(Boolean).join(", ")}
                  </p>
                </div>
                <Button variant="secondary" size="sm" onClick={() => setConfirmTarget(n)}>
                  Buy
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>

      <Dialog
        open={confirmTarget != null}
        onClose={() => setConfirmTarget(null)}
        title="Purchase phone number?"
        subtitle={confirmTarget?.phone_number}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirmTarget(null)}>
              Cancel
            </Button>
            <Button disabled={purchase.isPending} onClick={confirmPurchase}>
              {purchase.isPending ? "Purchasing…" : "Buy"}
            </Button>
          </>
        }
      >
        <p className="text-sm text-gray-600">
          This will charge your Twilio account
          {priceLabel ? (
            <>
              {" "}
              approximately <strong>{priceLabel}</strong>
            </>
          ) : (
            " (monthly price varies by account)"
          )}
          . Voice webhook is configured when you assign this number to a call source.
        </p>
      </Dialog>

      <Dialog
        open={releaseTarget != null}
        onClose={() => setReleaseTarget(null)}
        title="Release phone number?"
        subtitle={releaseTarget?.phone}
        footer={
          <>
            <Button variant="secondary" onClick={() => setReleaseTarget(null)}>
              Cancel
            </Button>
            <Button variant="secondary" disabled={release.isPending} onClick={confirmRelease}>
              {release.isPending ? "Releasing…" : "Release"}
            </Button>
          </>
        }
      >
        <p className="text-sm text-gray-600">
          This removes the number from your Twilio account. You will stop being billed for it.
        </p>
      </Dialog>
    </div>
  );
}
