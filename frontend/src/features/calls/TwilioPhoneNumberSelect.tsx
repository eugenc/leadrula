import { useEffect, useRef, useState } from "react";
import { Input } from "@/components/ui/input";
import { useTwilioPhoneNumbers } from "@/features/integrations/hooks";
import type { TwilioPhoneNumber } from "@/types";

function numberLabel(n: TwilioPhoneNumber): string {
  if (n.friendly_name && n.friendly_name !== n.phone_number) {
    return `${n.phone_number} (${n.friendly_name})`;
  }
  return n.phone_number;
}

export function TwilioPhoneNumberSelect({
  connectionId,
  valueSid,
  valueNumber,
  onChange,
  disabled,
}: {
  connectionId: number | null;
  valueSid: string;
  valueNumber: string;
  onChange: (sid: string, phoneNumber: string) => void;
  disabled?: boolean;
}) {
  const [query, setQuery] = useState(valueNumber);
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const { data, isLoading, isError } = useTwilioPhoneNumbers(connectionId);

  useEffect(() => {
    setQuery(valueNumber);
  }, [valueNumber, valueSid]);

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  const numbers = data ?? [];
  const q = query.trim().toLowerCase();
  const filtered =
    q === ""
      ? numbers
      : numbers.filter(
          (n) =>
            n.phone_number.toLowerCase().includes(q) ||
            n.friendly_name.toLowerCase().includes(q) ||
            n.sid.toLowerCase().includes(q)
        );

  const showPanel = open && connectionId != null && connectionId > 0;

  function select(n: TwilioPhoneNumber) {
    onChange(n.sid, n.phone_number);
    setQuery(n.phone_number);
    setOpen(false);
  }

  return (
    <div ref={containerRef} className="relative">
      <Input
        value={query}
        disabled={disabled || !connectionId}
        placeholder={connectionId ? "Search phone numbers…" : "Select a Twilio account first"}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
          if (!e.target.value.trim()) onChange("", "");
        }}
        onFocus={() => setOpen(true)}
      />
      {showPanel && (
        <div className="absolute z-50 mt-1 max-h-56 w-full overflow-auto rounded-md border border-gray-200 bg-white py-1 shadow-lg">
          {isLoading && <p className="px-3 py-2 text-sm text-gray-500">Loading numbers…</p>}
          {isError && (
            <p className="px-3 py-2 text-sm text-red-600">Failed to load Twilio numbers.</p>
          )}
          {!isLoading && !isError && filtered.length === 0 && (
            <p className="px-3 py-2 text-sm text-gray-500">No voice-capable numbers found.</p>
          )}
          {filtered.map((n) => (
            <button
              key={n.sid}
              type="button"
              className={`block w-full px-3 py-2 text-left text-sm hover:bg-gray-50 ${
                n.sid === valueSid ? "bg-jade-50 text-jade-800" : "text-gray-800"
              }`}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => select(n)}
            >
              {numberLabel(n)}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
