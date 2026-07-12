import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { get, ns } from "@/lib/api";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

interface LeadRow {
  public_id: string;
  first_name?: string;
  last_name?: string;
  phone?: string;
  city?: string;
  state?: string;
}

// The leads endpoint returns either an array or a paginated { items } envelope.
function normalize(raw: unknown): LeadRow[] {
  if (Array.isArray(raw)) return raw as LeadRow[];
  const items = (raw as { items?: LeadRow[] })?.items;
  return items ?? [];
}

export function LeadSharePicker({ onClose, onPick }: { onClose: () => void; onPick: (leadId: string) => void }) {
  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query), 300);
    return () => clearTimeout(t);
  }, [query]);

  const { data, isLoading } = useQuery({
    queryKey: ["messaging-lead-picker", debounced],
    queryFn: async () => {
      const qs = debounced.trim() ? `?q=${encodeURIComponent(debounced.trim())}&limit=20` : "?limit=20";
      return normalize(await get<unknown>(`${ns()}/leads${qs}`));
    },
  });

  return (
    <Dialog open onClose={onClose} title="Share a lead" subtitle="Only leads the recipient can access can be shared.">
      <Input
        autoFocus
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search leads by name"
        className="mb-2"
      />
      <div className="max-h-64 overflow-y-auto">
        {isLoading && <p className="py-4 text-center text-sm text-gray-400">Loading…</p>}
        {!isLoading && (data?.length ?? 0) === 0 && (
          <p className="py-4 text-center text-sm text-gray-400">No leads found.</p>
        )}
        {data?.map((l) => {
          const name = [l.first_name, l.last_name].filter(Boolean).join(" ") || "Unnamed lead";
          return (
            <button
              key={l.public_id}
              type="button"
              onClick={() => onPick(l.public_id)}
              className="flex w-full flex-col items-start rounded-md px-2 py-1.5 text-left hover:bg-gray-50"
            >
              <span className="text-sm font-medium text-gray-800">{name}</span>
              <span className="text-xs text-gray-400">
                {[l.phone, l.city, l.state].filter(Boolean).join(" · ")}
              </span>
            </button>
          );
        })}
      </div>
    </Dialog>
  );
}
