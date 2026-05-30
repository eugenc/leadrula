import { useState, type KeyboardEvent } from "react";
import { X } from "lucide-react";
import { Input, Label } from "@/components/ui/input";
import { Badge } from "@/components/ui/misc";
import { useTagSuggestions, useUpdateLead } from "./hooks";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";

function normalizeTags(tags: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of tags) {
    const t = raw.trim();
    if (!t) continue;
    const key = t.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(t);
  }
  return out;
}

export function LeadTagsEditor({ leadId, tags }: { leadId: number; tags: string[] }) {
  const [input, setInput] = useState("");
  const update = useUpdateLead();
  const { data: suggestions } = useTagSuggestions();

  function save(next: string[]) {
    update.mutate(
      { leadId, body: { tags: normalizeTags(next) } },
      {
        onError: (err) => toast.error(apiError(err).message),
      }
    );
  }

  function addTag(raw: string) {
    const parts = raw.split(",").map((s) => s.trim()).filter(Boolean);
    if (!parts.length) return;
    save(normalizeTags([...tags, ...parts]));
    setInput("");
  }

  function removeTag(tag: string) {
    save(tags.filter((t) => t !== tag));
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      addTag(input);
    }
  }

  return (
    <div>
      <Label>Tags</Label>
      <div className="mt-1 flex flex-wrap gap-1.5">
        {tags.map((tag) => (
          <Badge key={tag} variant="default" className="gap-1 pr-1">
            {tag}
            <button
              type="button"
              onClick={() => removeTag(tag)}
              className="rounded-full p-0.5 hover:bg-gray-200"
              aria-label={`Remove tag ${tag}`}
            >
              <X className="h-3 w-3" />
            </button>
          </Badge>
        ))}
      </div>
      <Input
        value={input}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={onKeyDown}
        onBlur={() => input.trim() && addTag(input)}
        placeholder="Add tag…"
        list="lead-tag-suggestions"
        className="mt-2"
      />
      <datalist id="lead-tag-suggestions">
        {(suggestions ?? [])
          .filter((s) => !tags.some((t) => t.toLowerCase() === s.toLowerCase()))
          .map((s) => (
            <option key={s} value={s} />
          ))}
      </datalist>
    </div>
  );
}

export function LeadTagBadges({ tags, limit }: { tags: string[]; limit?: number }) {
  const shown = limit ? tags.slice(0, limit) : tags;
  const extra = limit && tags.length > limit ? tags.length - limit : 0;
  if (!shown.length) return null;
  return (
    <div className="flex flex-wrap gap-1">
      {shown.map((tag) => (
        <Badge key={tag} variant="default" className="text-[10px]">
          {tag}
        </Badge>
      ))}
      {extra > 0 && (
        <span className="text-xs text-gray-400">+{extra}</span>
      )}
    </div>
  );
}
