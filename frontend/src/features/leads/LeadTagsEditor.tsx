import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useState,
  type KeyboardEvent,
} from "react";
import { X } from "lucide-react";
import { Input, Label } from "@/components/ui/input";
import { Badge } from "@/components/ui/misc";
import { cn } from "@/lib/utils";
import { useTagSuggestions, useUpdateLead } from "./hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";

export function normalizeTags(tags: string[]): string[] {
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

function parseTagInput(raw: string): string[] {
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

export type TagsInputHandle = { commitPending: () => string[] };

type TagsInputProps = {
  tags: string[];
  onChange: (tags: string[]) => void;
  suggestions?: string[];
  listId?: string;
  className?: string;
};

export const TagsInput = forwardRef<TagsInputHandle, TagsInputProps>(function TagsInput(
  { tags, onChange, suggestions, listId, className },
  ref
) {
  const [input, setInput] = useState("");

  function mergeInput(raw: string): string[] | null {
    const parts = parseTagInput(raw);
    if (!parts.length) return null;
    return normalizeTags([...tags, ...parts]);
  }

  function addTag(raw: string) {
    const next = mergeInput(raw);
    if (!next) return;
    onChange(next);
    setInput("");
  }

  function removeTag(tag: string) {
    onChange(tags.filter((t) => t !== tag));
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      addTag(input);
    }
  }

  useImperativeHandle(ref, () => ({
    commitPending: () => {
      const next = mergeInput(input);
      if (next) {
        onChange(next);
        setInput("");
        return next;
      }
      return tags;
    },
  }));

  const filteredSuggestions = (suggestions ?? []).filter(
    (s) => !tags.some((t) => t.toLowerCase() === s.toLowerCase())
  );

  return (
    <div className={className}>
      {tags.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
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
      )}
      <Input
        value={input}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={onKeyDown}
        onBlur={() => input.trim() && addTag(input)}
        placeholder="Add tag…"
        list={listId}
        className={cn(tags.length > 0 && "mt-2")}
      />
      {listId && filteredSuggestions.length > 0 && (
        <datalist id={listId}>
          {filteredSuggestions.map((s) => (
            <option key={s} value={s} />
          ))}
        </datalist>
      )}
    </div>
  );
});

export function LeadTagsEditor({ leadId, tags }: { leadId: number; tags: string[] }) {
  const [localTags, setLocalTags] = useState(tags);
  const update = useUpdateLead();
  const { data: suggestions } = useTagSuggestions();

  useEffect(() => {
    setLocalTags(tags);
  }, [tags]);

  function handleChange(next: string[]) {
    const normalized = normalizeTags(next);
    setLocalTags(normalized);
    update.mutate(
      { leadId, body: { tags: normalized } },
      {
        onError: (err) => toast.error(errorMessage(err)),
      }
    );
  }

  return (
    <div>
      <Label>Tags</Label>
      <TagsInput
        tags={localTags}
        onChange={handleChange}
        suggestions={suggestions}
        listId="lead-tag-suggestions"
        className="mt-1"
      />
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
