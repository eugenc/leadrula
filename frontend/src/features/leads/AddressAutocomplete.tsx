import { useEffect, useMemo, useRef, useState } from "react";
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
  address_place_id: string;
};

type PlainFields = {
  address: string;
  city: string;
  state: string;
  zip: string;
};

function newSessionToken() {
  return crypto.randomUUID();
}

function formatAddressLine(fields: PlainFields) {
  const parts = [fields.address, fields.city, fields.state, fields.zip].filter(Boolean);
  return parts.join(", ");
}

export function AddressAutocomplete({
  address = "",
  city = "",
  state = "",
  zip = "",
  onPlainChange,
  onFieldBlur,
  onSelect,
  disabled,
}: {
  address?: string;
  city?: string;
  state?: string;
  zip?: string;
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

  const displayValue = useMemo(() => formatAddressLine({ address, city, state, zip }), [address, city, state, zip]);

  useEffect(() => {
    if (!connected) return;
    setQuery(displayValue);
  }, [connected, displayValue]);

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
        address_place_id: details.place_id,
      });
      setQuery(formatAddressLine(details));
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
          <ul className="absolute z-20 mt-1 max-h-56 w-full overflow-auto rounded-md border border-gray-100 bg-white py-1 shadow-lg">
            {suggestions.map((item) => (
              <li key={item.place_id}>
                <button
                  type="button"
                  className="w-full px-3 py-2 text-left text-sm hover:bg-gray-50"
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
      {(city || state || zip) && (
        <p className="text-xs text-gray-500">
          {[city, state, zip].filter(Boolean).join(", ")}
        </p>
      )}
    </div>
  );
}

function PlainAddressFields({
  address,
  city,
  state,
  zip,
  disabled,
  onChange,
  onFieldBlur,
}: {
  address: string;
  city: string;
  state: string;
  zip: string;
  disabled?: boolean;
  onChange?: (fields: PlainFields) => void;
  onFieldBlur?: (key: keyof PlainFields) => void;
}) {
  function setField(key: keyof PlainFields, value: string) {
    onChange?.({ address, city, state, zip, [key]: value });
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
    </>
  );
}

export function formatLeadAddress(lead: {
  address?: string | null;
  city?: string | null;
  state?: string | null;
  zip?: string | null;
}) {
  return formatAddressLine({
    address: lead.address ?? "",
    city: lead.city ?? "",
    state: lead.state ?? "",
    zip: lead.zip ?? "",
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
