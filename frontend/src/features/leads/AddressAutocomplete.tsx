import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { Input, Label } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { useAuthStore } from "@/store/authStore";
import {
  fetchGoogleMapsAutocomplete,
  fetchGoogleMapsPlaceDetails,
  useGoogleMapsStatus,
  type GoogleMapsSuggestion,
} from "@/features/integrations/hooks";
import { cn } from "@/lib/utils";

export type ValidatedAddress = {
  address: string;
  city: string;
  state: string;
  zip: string;
  country: string;
  address_place_id: string;
};

type PlainFields = {
  address: string;
  city: string;
  state: string;
  zip: string;
  country: string;
};

function newSessionToken() {
  return crypto.randomUUID();
}

function formatAddressLine(fields: PlainFields) {
  const parts = [fields.address, fields.city, fields.state, fields.zip, fields.country].filter(Boolean);
  return parts.join(", ");
}

export function AddressAutocomplete({
  address = "",
  city = "",
  state = "",
  zip = "",
  country = "",
  onPlainChange,
  onFieldBlur,
  onSelect,
  disabled,
}: {
  address?: string;
  city?: string;
  state?: string;
  zip?: string;
  country?: string;
  onPlainChange?: (fields: PlainFields) => void;
  onFieldBlur?: (key: keyof PlainFields) => void;
  onSelect?: (value: ValidatedAddress) => void;
  disabled?: boolean;
}) {
  const accountType = useAuthStore((s) => s.user?.account_type);
  const integrationsPath = accountType === "buyer" ? "/b/integrations" : "/p/integrations";
  const { data: status, isLoading: statusLoading } = useGoogleMapsStatus();
  const connected = status?.connected === true;

  const [query, setQuery] = useState("");
  const [suggestions, setSuggestions] = useState<GoogleMapsSuggestion[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [sessionToken, setSessionToken] = useState(newSessionToken);
  const [selecting, setSelecting] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const hasParsedFields = !!(city || state || zip || country);

  useEffect(() => {
    if (!connected) return;
    setQuery(address);
  }, [connected, address]);

  useEffect(() => {
    if (!connected || !open) return;
    const trimmed = query.trim();
    if (trimmed.length < 3) {
      setSuggestions([]);
      return;
    }
    const timer = window.setTimeout(async () => {
      setLoading(true);
      try {
        const items = await fetchGoogleMapsAutocomplete(trimmed, sessionToken);
        setSuggestions(items);
      } catch {
        setSuggestions([]);
      } finally {
        setLoading(false);
      }
    }, 250);
    return () => window.clearTimeout(timer);
  }, [connected, query, open, sessionToken]);

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (!containerRef.current?.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, []);

  async function pickSuggestion(item: GoogleMapsSuggestion) {
    setSelecting(true);
    setOpen(false);
    try {
      const details = await fetchGoogleMapsPlaceDetails(item.place_id);
      onSelect?.({
        address: details.address,
        city: details.city,
        state: details.state,
        zip: details.zip,
        country: details.country,
        address_place_id: details.place_id,
      });
      setQuery(details.address);
      setSessionToken(newSessionToken());
      setSuggestions([]);
    } finally {
      setSelecting(false);
    }
  }

  if (statusLoading) {
    return <Spinner className="h-5 w-5" />;
  }

  if (!connected) {
    return (
      <div className="flex flex-col gap-2.5">
        <p className="text-xs text-gray-500">
          Connect Google Maps in{" "}
          <Link to={integrationsPath} className="text-indigo-600 hover:underline">
            Integrations
          </Link>{" "}
          to validate addresses, or enter them manually below.
        </p>
        <PlainAddressFields
          address={address}
          city={city}
          state={state}
          zip={zip}
          country={country}
          disabled={disabled}
          onChange={onPlainChange}
          onFieldBlur={onFieldBlur}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2.5">
      <div ref={containerRef} className="relative">
        <Label>Address</Label>
        <Input
          value={query}
          disabled={disabled || selecting}
          placeholder="Start typing an address…"
          onFocus={() => setOpen(true)}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
          }}
        />
        {(loading || selecting) && (
          <div className="absolute right-2 top-8">
            <Spinner className="h-4 w-4" />
          </div>
        )}
        {open && suggestions.length > 0 && (
          <ul className="absolute z-20 mt-1 max-h-56 w-full overflow-auto rounded-md border border-gray-100 bg-surface-card py-1 shadow-lg">
            {suggestions.map((item) => (
              <li key={item.place_id}>
                <button
                  type="button"
                  className="w-full px-3 py-2 text-left text-sm hover:bg-surface-hover"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => pickSuggestion(item)}
                >
                  <div className="font-medium text-gray-800">{item.main_text || item.description}</div>
                  {item.secondary_text && (
                    <div className="text-xs text-gray-500">{item.secondary_text}</div>
                  )}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
      {hasParsedFields && (
        <AddressFieldsPreview city={city} state={state} zip={zip} country={country} />
      )}
    </div>
  );
}

function AddressFieldsPreview({
  city,
  state,
  zip,
  country,
}: {
  city: string;
  state: string;
  zip: string;
  country: string;
}) {
  const items = [
    { label: "City", value: city },
    { label: "State", value: state },
    { label: "Zip", value: zip },
    { label: "Country", value: country },
  ].filter((item) => item.value);

  if (items.length === 0) return null;

  return (
    <div className="grid grid-cols-2 gap-2.5">
      {items.map((item) => (
        <div key={item.label}>
          <Label>{item.label}</Label>
          <div className="mt-1 rounded-md border border-gray-100 bg-gray-50 px-2.5 py-1.5 text-sm text-gray-700 dark:bg-gray-800/50 dark:text-gray-300">
            {item.value}
          </div>
        </div>
      ))}
    </div>
  );
}

function PlainAddressFields({
  address,
  city,
  state,
  zip,
  country,
  disabled,
  onChange,
  onFieldBlur,
}: {
  address: string;
  city: string;
  state: string;
  zip: string;
  country: string;
  disabled?: boolean;
  onChange?: (fields: PlainFields) => void;
  onFieldBlur?: (key: keyof PlainFields) => void;
}) {
  function setField(key: keyof PlainFields, value: string) {
    onChange?.({ address, city, state, zip, country, [key]: value });
  }

  return (
    <>
      <div>
        <Label>Address</Label>
        <Input
          value={address}
          disabled={disabled}
          onChange={(e) => setField("address", e.target.value)}
          onBlur={() => onFieldBlur?.("address")}
        />
      </div>
      <div>
        <Label>City</Label>
        <Input
          value={city}
          disabled={disabled}
          onChange={(e) => setField("city", e.target.value)}
          onBlur={() => onFieldBlur?.("city")}
        />
      </div>
      <div>
        <Label>State</Label>
        <Input
          value={state}
          disabled={disabled}
          onChange={(e) => setField("state", e.target.value)}
          onBlur={() => onFieldBlur?.("state")}
        />
      </div>
      <div>
        <Label>Zip</Label>
        <Input
          value={zip}
          disabled={disabled}
          onChange={(e) => setField("zip", e.target.value)}
          onBlur={() => onFieldBlur?.("zip")}
        />
      </div>
      <div>
        <Label>Country</Label>
        <Input
          value={country}
          disabled={disabled}
          onChange={(e) => setField("country", e.target.value)}
          onBlur={() => onFieldBlur?.("country")}
        />
      </div>
    </>
  );
}

export function formatLeadAddress(lead: {
  address?: string | null;
  city?: string | null;
  state?: string | null;
  zip?: string | null;
  country?: string | null;
}) {
  return formatAddressLine({
    address: lead.address ?? "",
    city: lead.city ?? "",
    state: lead.state ?? "",
    zip: lead.zip ?? "",
    country: lead.country ?? "",
  });
}

export function ValidatedAddressLink({
  formatted,
  onClick,
  className,
}: {
  formatted: string;
  onClick: () => void;
  className?: string;
}) {
  if (!formatted) return null;
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "inline-flex items-center gap-1.5 text-left text-sm text-indigo-600 hover:underline",
        className
      )}
    >
      {formatted}
    </button>
  );
}
