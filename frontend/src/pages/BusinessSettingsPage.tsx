import { useEffect, useState } from "react";
import { Navigate } from "react-router-dom";
import { useMe, useUpdateMyAccount } from "@/features/leads/hooks";
import { useAuthStore } from "@/store/authStore";
import { Card } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { TIMEZONES } from "@/lib/timezones";
import type { AccountType, Me } from "@/types";

function HandlerIDRow({
  handlerId,
  accountType,
}: {
  handlerId: string;
  accountType: "buyer" | "publisher";
}) {
  const label = accountType === "buyer" ? "Buyer ID" : "Publisher ID";
  return (
    <div className="flex items-center justify-between border-t border-gray-100 pt-4">
      <div>
        <span className="text-sm text-gray-400">{label}</span>
        <p className="text-xs text-gray-400">Share this so others can link your account</p>
      </div>
      <div className="flex items-center gap-2">
        <code className="text-sm font-semibold text-gray-800">{handlerId}</code>
        <Button
          variant="secondary"
          className="h-7 px-2 text-xs"
          onClick={() => {
            void navigator.clipboard.writeText(handlerId).then(() => toast.success(`Copied ${label}`));
          }}
        >
          Copy
        </Button>
      </div>
    </div>
  );
}

type BusinessForm = {
  name: string;
  website: string;
  timezone: string;
  contact_email: string;
  phone: string;
  address_line1: string;
  address_line2: string;
  city: string;
  state: string;
  postal_code: string;
  country: string;
};

function accountToForm(account: Me["account"]): BusinessForm {
  return {
    name: account.name ?? "",
    website: account.website ?? "",
    timezone: account.timezone ?? "America/Toronto",
    contact_email: account.contact_email ?? "",
    phone: account.phone ?? "",
    address_line1: account.address_line1 ?? "",
    address_line2: account.address_line2 ?? "",
    city: account.city ?? "",
    state: account.state ?? "",
    postal_code: account.postal_code ?? "",
    country: account.country ?? "",
  };
}

function buildPatch(form: BusinessForm, saved: BusinessForm): Partial<Me["account"]> {
  const body: Partial<Me["account"]> = {};
  const trimmedName = form.name.trim();
  if (trimmedName !== saved.name) body.name = trimmedName;
  if (form.website.trim() !== saved.website) body.website = form.website.trim();
  if (form.timezone !== saved.timezone) body.timezone = form.timezone;
  if (form.contact_email.trim() !== saved.contact_email) body.contact_email = form.contact_email.trim();
  if (form.phone.trim() !== saved.phone) body.phone = form.phone.trim();
  if (form.address_line1.trim() !== saved.address_line1) body.address_line1 = form.address_line1.trim();
  if (form.address_line2.trim() !== saved.address_line2) body.address_line2 = form.address_line2.trim();
  if (form.city.trim() !== saved.city) body.city = form.city.trim();
  if (form.state.trim() !== saved.state) body.state = form.state.trim();
  if (form.postal_code.trim() !== saved.postal_code) body.postal_code = form.postal_code.trim();
  if (form.country.trim() !== saved.country) body.country = form.country.trim();
  return body;
}

export function BusinessSettingsPage() {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const accountType = user?.account_type;
  const prefix = accountType === "publisher" ? "/p" : "/b";
  const { data: me } = useMe();
  const updateAccount = useUpdateMyAccount();

  const saved = accountToForm(
    me?.account ?? {
      id: "",
      handler_id: "",
      type: "buyer",
      name: "",
      timezone: "America/Toronto",
      website: "",
      contact_email: "",
      phone: "",
      address_line1: "",
      address_line2: "",
      city: "",
      state: "",
      postal_code: "",
      country: "",
    }
  );

  const [form, setForm] = useState<BusinessForm>(saved);

  useEffect(() => {
    if (me?.account) setForm(accountToForm(me.account));
  }, [me?.account]);

  if (!isAdmin) {
    return <Navigate to={`${prefix}/settings`} replace />;
  }

  if (accountType !== "buyer" && accountType !== "publisher") {
    return <Navigate to={`${prefix}/settings`} replace />;
  }

  const patch = buildPatch(form, saved);
  const unchanged = Object.keys(patch).length === 0;
  const invalid = !form.name.trim();

  const set = (key: keyof BusinessForm) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setForm((f) => ({ ...f, [key]: e.target.value }));

  const save = () => {
    if (unchanged || invalid) return;
    updateAccount.mutate(patch, {
      onSuccess: () => toast.success("Business profile updated"),
      onError: (err) => toast.error(errorMessage(err)),
    });
  };

  const handlerId = me?.account.handler_id;
  const typedAccountType = accountType as AccountType;

  return (
    <div className="max-w-xl space-y-4">
      <Card className="p-5">
        <div className="flex flex-col gap-2.5">
          <div>
            <Label>Company name</Label>
            <Input value={form.name} onChange={set("name")} />
          </div>
          <div>
            <Label>Website</Label>
            <Input value={form.website} onChange={set("website")} placeholder="https://example.com" />
          </div>
          <div>
            <Label>Timezone</Label>
            <Select value={form.timezone} onChange={set("timezone")}>
              {TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Business email</Label>
            <Input
              type="email"
              value={form.contact_email}
              onChange={set("contact_email")}
              placeholder="contact@company.com"
            />
          </div>
          <div>
            <Label>Phone</Label>
            <Input value={form.phone} onChange={set("phone")} />
          </div>
          <div>
            <Label>Address line 1</Label>
            <Input value={form.address_line1} onChange={set("address_line1")} />
          </div>
          <div>
            <Label>Address line 2</Label>
            <Input value={form.address_line2} onChange={set("address_line2")} placeholder="Optional" />
          </div>
          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <Label>City</Label>
              <Input value={form.city} onChange={set("city")} />
            </div>
            <div>
              <Label>State / province</Label>
              <Input value={form.state} onChange={set("state")} />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <Label>Postal code</Label>
              <Input value={form.postal_code} onChange={set("postal_code")} />
            </div>
            <div>
              <Label>Country</Label>
              <Input value={form.country} onChange={set("country")} />
            </div>
          </div>
        </div>

        {handlerId ? (
          <HandlerIDRow handlerId={handlerId} accountType={typedAccountType as "buyer" | "publisher"} />
        ) : null}

        <div className="mt-4 flex justify-end">
          <Button disabled={unchanged || invalid || updateAccount.isPending} onClick={save}>
            Save
          </Button>
        </div>
      </Card>
    </div>
  );
}
