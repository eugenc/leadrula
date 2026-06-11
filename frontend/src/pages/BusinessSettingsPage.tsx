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
import {
  COUNTRIES,
  countryCodeForName,
  countryNameForCode,
  stateMatchesSubdivision,
  subdivisionsForCountryCode,
} from "@/lib/countries";
import type { AccountType, Me } from "@/types";

const fieldLabelClass = "text-sm font-medium text-gray-400";
const fieldInputClass = "text-sm";

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
  countrySelect: string;
  countryOther: string;
};

type ApiAddress = Pick<BusinessForm, "city" | "state" | "postal_code"> & { country: string };

function effectiveCountry(form: Pick<BusinessForm, "countrySelect" | "countryOther">): string {
  if (form.countrySelect === "OTHER") return form.countryOther.trim();
  return countryNameForCode(form.countrySelect);
}

function formToApiAddress(form: BusinessForm): ApiAddress {
  return {
    city: form.city.trim(),
    state: form.state.trim(),
    postal_code: form.postal_code.trim(),
    country: effectiveCountry(form),
  };
}

function accountToForm(account: Me["account"]): BusinessForm {
  const countryName = account.country ?? "";
  const code = countryCodeForName(countryName);
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
    countrySelect: !countryName ? "" : code || "OTHER",
    countryOther: code === "OTHER" ? countryName : "",
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

  const address = formToApiAddress(form);
  const savedAddress = formToApiAddress(saved);
  if (address.city !== savedAddress.city) body.city = address.city;
  if (address.state !== savedAddress.state) body.state = address.state;
  if (address.postal_code !== savedAddress.postal_code) body.postal_code = address.postal_code;
  if (address.country !== savedAddress.country) body.country = address.country;

  return body;
}

const emptyAccount: Me["account"] = {
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
};

export function BusinessSettingsPage() {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const accountType = user?.account_type;
  const prefix = accountType === "publisher" ? "/p" : "/b";
  const { data: me } = useMe();
  const updateAccount = useUpdateMyAccount();

  const saved = accountToForm(me?.account ?? emptyAccount);
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

  const onCountryChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const code = e.target.value;
    setForm((f) => {
      const subdivisions = subdivisionsForCountryCode(code);
      const keepState = stateMatchesSubdivision(f.state, subdivisions);
      return {
        ...f,
        countrySelect: code,
        state: keepState ? f.state : "",
      };
    });
  };

  const save = () => {
    if (unchanged || invalid) return;
    updateAccount.mutate(patch, {
      onSuccess: () => toast.success("Business profile updated"),
      onError: (err) => toast.error(errorMessage(err)),
    });
  };

  const handlerId = me?.account.handler_id;
  const typedAccountType = accountType as AccountType;
  const countryIsOther = form.countrySelect === "OTHER";
  const subdivisions = subdivisionsForCountryCode(form.countrySelect);

  return (
    <div className="max-w-xl space-y-4">
      <Card className="p-5">
        <div className="flex flex-col gap-2">
          <div>
            <Label className={fieldLabelClass}>Company name</Label>
            <Input className={fieldInputClass} value={form.name} onChange={set("name")} />
          </div>
          <div>
            <Label className={fieldLabelClass}>Website</Label>
            <Input
              className={fieldInputClass}
              value={form.website}
              onChange={set("website")}
              placeholder="https://example.com"
            />
          </div>
          <div>
            <Label className={fieldLabelClass}>Timezone</Label>
            <Select className={fieldInputClass} value={form.timezone} onChange={set("timezone")}>
              {TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label className={fieldLabelClass}>Business email</Label>
            <Input
              className={fieldInputClass}
              type="email"
              value={form.contact_email}
              onChange={set("contact_email")}
              placeholder="contact@company.com"
            />
          </div>
          <div>
            <Label className={fieldLabelClass}>Phone</Label>
            <Input className={fieldInputClass} value={form.phone} onChange={set("phone")} />
          </div>
          <div>
            <Label className={fieldLabelClass}>Address line 1</Label>
            <Input className={fieldInputClass} value={form.address_line1} onChange={set("address_line1")} />
          </div>
          <div>
            <Label className={fieldLabelClass}>Address line 2</Label>
            <Input
              className={fieldInputClass}
              value={form.address_line2}
              onChange={set("address_line2")}
              placeholder="Optional"
            />
          </div>
          <div>
            <Label className={fieldLabelClass}>City</Label>
            <Input className={fieldInputClass} value={form.city} onChange={set("city")} />
          </div>
          <div>
            <Label className={fieldLabelClass}>Country</Label>
            <Select className={fieldInputClass} value={form.countrySelect} onChange={onCountryChange}>
              <option value="">Select country</option>
              {COUNTRIES.map((c) => (
                <option key={c.code} value={c.code}>
                  {c.name}
                </option>
              ))}
            </Select>
          </div>
          {countryIsOther ? (
            <div>
              <Label className={fieldLabelClass}>Country name</Label>
              <Input
                className={fieldInputClass}
                value={form.countryOther}
                onChange={set("countryOther")}
                placeholder="Enter country"
              />
            </div>
          ) : null}
          <div>
            <Label className={fieldLabelClass}>State / province</Label>
            {subdivisions ? (
              <Select className={fieldInputClass} value={form.state} onChange={set("state")}>
                <option value="">Select state / province</option>
                {subdivisions.map((s) => (
                  <option key={s.code} value={s.name}>
                    {s.name}
                  </option>
                ))}
              </Select>
            ) : (
              <Input className={fieldInputClass} value={form.state} onChange={set("state")} />
            )}
          </div>
          <div>
            <Label className={fieldLabelClass}>Postal code</Label>
            <Input className={fieldInputClass} value={form.postal_code} onChange={set("postal_code")} />
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
