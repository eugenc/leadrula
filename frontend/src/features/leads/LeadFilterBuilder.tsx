import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { FilterSelect } from "@/components/ui/input";
import { FILTER_FIELDS, type FilterCondition } from "./leadsViews";
import { usePipelines, useStages, useUsers } from "./hooks";

interface Props {
  conditions: FilterCondition[];
  onChange: (conditions: FilterCondition[]) => void;
}

const STATUS_OPTIONS = [
  { value: "distributed", label: "Distributed" },
  { value: "returned", label: "Returned" },
  { value: "review", label: "In Review" },
  { value: "closed", label: "Closed" },
];

function emptyCondition(): FilterCondition {
  const first = FILTER_FIELDS[0]!;
  return { field: first.field, op: first.ops[0]!.op, value: "me" };
}

export function LeadFilterBuilder({ conditions, onChange }: Props) {
  const { data: pipelines } = usePipelines();
  const { data: users } = useUsers();
  const pipelineId = conditions.find((c) => c.field === "pipeline_id")?.value;
  const { data: stages } = useStages(typeof pipelineId === "number" ? pipelineId : Number(pipelineId) || undefined);

  function update(i: number, patch: Partial<FilterCondition>) {
    const next = conditions.map((c, idx) => (idx === i ? { ...c, ...patch } : c));
    onChange(next);
  }

  function onFieldChange(i: number, field: string) {
    const def = FILTER_FIELDS.find((f) => f.field === field)!;
    const op = def.ops[0]!;
    const value = op.valueType === "user" ? "me" : op.valueType === "date" ? "today" : undefined;
    update(i, { field, op: op.op, value });
  }

  function addCondition() {
    onChange([...conditions, emptyCondition()]);
  }

  function removeCondition(i: number) {
    onChange(conditions.filter((_, idx) => idx !== i));
  }

  const activeUsers = (users ?? []).filter((u) => u.status === "active");

  return (
    <div className="flex flex-col gap-3">
      {conditions.length === 0 && (
        <p className="text-sm text-gray-400">No conditions — all leads match.</p>
      )}
      {conditions.map((c, i) => {
        const fieldDef = FILTER_FIELDS.find((f) => f.field === c.field) ?? FILTER_FIELDS[0]!;
        const opDef = fieldDef.ops.find((o) => o.op === c.op) ?? fieldDef.ops[0]!;
        return (
          <div key={i} className="flex flex-wrap items-center gap-2">
            {i > 0 && <span className="text-xs font-medium uppercase text-gray-400">and</span>}
            <FilterSelect
              value={c.field}
              onChange={(e) => onFieldChange(i, e.target.value)}
              className="w-36"
            >
              {FILTER_FIELDS.map((f) => (
                <option key={f.field} value={f.field}>
                  {f.label}
                </option>
              ))}
            </FilterSelect>
            <FilterSelect
              value={c.op}
              onChange={(e) => update(i, { op: e.target.value })}
              className="w-32"
            >
              {fieldDef.ops.map((o) => (
                <option key={o.op} value={o.op}>
                  {o.label}
                </option>
              ))}
            </FilterSelect>
            {opDef.needsValue && (
              <>
                {opDef.valueType === "user" && (
                  <FilterSelect
                    value={String(c.value ?? "me")}
                    onChange={(e) => update(i, { value: e.target.value === "me" ? "me" : Number(e.target.value) })}
                    className="w-40"
                  >
                    <option value="me">Me</option>
                    {activeUsers.map((u) => (
                      <option key={u.id} value={u.id}>
                        {u.full_name}
                      </option>
                    ))}
                  </FilterSelect>
                )}
                {opDef.valueType === "date" && (
                  <>
                    <FilterSelect
                      value={c.value === "today" || !c.value ? "today" : "custom"}
                      onChange={(e) =>
                        update(i, { value: e.target.value === "today" ? "today" : "" })
                      }
                      className="w-40"
                    >
                      <option value="today">Today</option>
                      <option value="custom">Specific date…</option>
                    </FilterSelect>
                    {c.value !== "today" && c.value !== undefined && (
                      <input
                        type="date"
                        value={typeof c.value === "string" ? c.value : ""}
                        onChange={(e) => update(i, { value: e.target.value })}
                        className="h-9 rounded-md border border-gray-200 px-2 text-sm"
                      />
                    )}
                  </>
                )}
                {opDef.valueType === "status" && (
                  <FilterSelect
                    value={String(c.value ?? "")}
                    onChange={(e) => update(i, { value: e.target.value })}
                    className="w-36"
                  >
                    <option value="">Select…</option>
                    {STATUS_OPTIONS.map((s) => (
                      <option key={s.value} value={s.value}>
                        {s.label}
                      </option>
                    ))}
                  </FilterSelect>
                )}
                {opDef.valueType === "pipeline" && (
                  <FilterSelect
                    value={String(c.value ?? "")}
                    onChange={(e) => update(i, { value: Number(e.target.value) })}
                    className="w-40"
                  >
                    <option value={0}>Select…</option>
                    {(pipelines ?? []).map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                  </FilterSelect>
                )}
                {opDef.valueType === "stage" && (
                  <FilterSelect
                    value={String(c.value ?? "")}
                    onChange={(e) => update(i, { value: Number(e.target.value) })}
                    className="w-40"
                  >
                    <option value={0}>Select…</option>
                    {(stages ?? []).map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </FilterSelect>
                )}
                {opDef.valueType === "text" && (
                  <input
                    type="text"
                    value={String(c.value ?? "")}
                    onChange={(e) => update(i, { value: e.target.value })}
                    placeholder="Value…"
                    className="h-9 w-40 rounded-md border border-gray-200 px-2 text-sm"
                  />
                )}
              </>
            )}
            <button
              type="button"
              onClick={() => removeCondition(i)}
              className="rounded p-1 text-gray-400 hover:text-danger"
              aria-label="Remove condition"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </div>
        );
      })}
      <Button type="button" variant="secondary" size="sm" onClick={addCondition} className="self-start">
        <Plus className="h-4 w-4" />
        Add condition
      </Button>
    </div>
  );
}
