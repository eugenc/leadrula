import { useRef, useState } from "react";
import { Plus } from "lucide-react";
import { Label, Input } from "@/components/ui/input";
import { Dropdown, DropdownItem, DropdownSearch } from "@/components/ui/dropdown";
import { useCustomFields } from "@/features/leads/hooks";
import { BUILTIN_FIELD_LABELS } from "@/features/leads/csvMapping";
import { GHL_TITLE_BUILTINS } from "@/features/integrations/ghlConstants";

type TemplateVariable = {
  key: string;
  label?: string;
  searchText: string;
};

export function GhlTitleTemplateEditor({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  const { data: customFields } = useCustomFields();
  const [inputEl, setInputEl] = useState<HTMLInputElement | null>(null);
  const [variablesOpen, setVariablesOpen] = useState(false);
  const [variableSearch, setVariableSearch] = useState("");
  const selectionRef = useRef({ start: 0, end: 0 });

  function saveSelection(el: HTMLInputElement) {
    selectionRef.current = { start: el.selectionStart ?? 0, end: el.selectionEnd ?? 0 };
  }

  function insertField(field: string) {
    if (inputEl) {
      const { start, end } = selectionRef.current;
      const before = value.slice(0, start);
      const after = value.slice(end);
      onChange(before + field + after);
      const cursor = start + field.length;
      selectionRef.current = { start: cursor, end: cursor };
      setTimeout(() => {
        inputEl.selectionStart = cursor;
        inputEl.selectionEnd = cursor;
        inputEl.focus();
      }, 0);
    } else {
      onChange(value + field);
    }
  }

  function selectVariable(field: string) {
    insertField(field);
    setVariablesOpen(false);
    setVariableSearch("");
  }

  const staticVariables: TemplateVariable[] = GHL_TITLE_BUILTINS.map((b) => ({
    key: `{{${b}}}`,
    label: BUILTIN_FIELD_LABELS[b] ?? b,
    searchText: `{{${b}}} ${BUILTIN_FIELD_LABELS[b] ?? b} ${b}`,
  }));

  const customVariables: TemplateVariable[] = (customFields ?? [])
    .filter((f) => f.is_active !== false)
    .map((f) => {
      const key = `{{custom:${f.id}}}`;
      return {
        key,
        label: f.name,
        searchText: `${key} ${f.name} ${f.field_key}`,
      };
    });

  const q = variableSearch.toLowerCase();
  const filteredStatic = staticVariables.filter((v) => v.searchText.toLowerCase().includes(q));
  const filteredCustom = customVariables.filter((v) => v.searchText.toLowerCase().includes(q));
  const hasResults = filteredStatic.length > 0 || filteredCustom.length > 0;

  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <Label>{label}</Label>
        <Dropdown
          open={variablesOpen}
          onClose={() => {
            setVariablesOpen(false);
            setVariableSearch("");
          }}
          align="right"
          className="max-h-48 min-w-[260px] overflow-y-auto"
          trigger={
            <button
              type="button"
              onClick={() => setVariablesOpen(!variablesOpen)}
              className="flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-800"
            >
              <Plus className="h-3 w-3" /> Add variable
            </button>
          }
        >
          <DropdownSearch
            value={variableSearch}
            onChange={setVariableSearch}
            placeholder="Search variables…"
          />
          {!hasResults ? (
            <p className="px-2.5 py-2 text-xs text-gray-400">No variables match</p>
          ) : (
            <>
              {filteredStatic.map((v) => (
                <DropdownItem key={v.key} onClick={() => selectVariable(v.key)} className="h-auto py-2">
                  <div className="text-xs text-gray-700">{v.label}</div>
                  <div className="font-mono text-xs text-gray-400">{v.key}</div>
                </DropdownItem>
              ))}
              {filteredStatic.length > 0 && filteredCustom.length > 0 && (
                <div className="my-1 border-t border-gray-100 px-2.5 py-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
                  Custom fields
                </div>
              )}
              {filteredCustom.map((v) => (
                <DropdownItem key={v.key} onClick={() => selectVariable(v.key)} className="h-auto py-2">
                  <div className="text-xs text-gray-700">{v.label}</div>
                  <div className="font-mono text-xs text-gray-400">{v.key}</div>
                </DropdownItem>
              ))}
            </>
          )}
        </Dropdown>
      </div>
      <Input
        ref={setInputEl}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onSelect={(e) => saveSelection(e.currentTarget)}
        onFocus={(e) => saveSelection(e.currentTarget)}
        onClick={(e) => saveSelection(e.currentTarget)}
        onKeyUp={(e) => saveSelection(e.currentTarget)}
        placeholder="e.g. Consultation: {{first_name}} {{last_name}}"
      />
      <p className="mt-1 text-xs text-gray-400">
        Use {"{{field}}"} placeholders — mix static text and variables.
      </p>
    </div>
  );
}
