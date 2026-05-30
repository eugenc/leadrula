import { useState } from "react";
import { Select } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/layout/IconButton";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { Plus, Trash2 } from "lucide-react";
import type { ReturnRule, Stage } from "@/types";

type Props = {
  buyerStages: Stage[];
  publisherStages: Stage[];
  rules: ReturnRule[];
  defaultReturnStageId: number;
  loading?: boolean;
  description?: string;
  onAdd: (buyerStageId: number, returnStageId: number) => void;
  onUpdate: (ruleId: number, buyerStageId: number, returnStageId: number) => void;
  onDelete: (ruleId: number) => void;
};

const DEFAULT_DESCRIPTION =
  "When a lead enters the From Stage on the buyer pipeline, it is returned to the To Stage on the publisher pipeline.";

function sortStages(stages: Stage[]) {
  return [...stages].sort((a, b) => a.position - b.position);
}

export function ContractReturnRulesEditor({
  buyerStages,
  publisherStages,
  rules,
  defaultReturnStageId,
  loading,
  description = DEFAULT_DESCRIPTION,
  onAdd,
  onUpdate,
  onDelete,
}: Props) {
  const [addFrom, setAddFrom] = useState(0);
  const [addTo, setAddTo] = useState(defaultReturnStageId);

  const sortedBuyer = sortStages(buyerStages);
  const sortedPublisher = sortStages(publisherStages);
  const usedFrom = new Set(rules.map((r) => r.buyer_stage_id));

  if (loading) {
    return <Spinner className="h-5 w-5" />;
  }

  if (sortedBuyer.length === 0 || sortedPublisher.length === 0) {
    return <EmptyState title="Pipeline stages are required to configure return rules." />;
  }

  const availableFrom = sortedBuyer.filter((s) => !usedFrom.has(s.id));

  return (
    <div>
      <p className="mb-3 text-xs text-gray-400">{description}</p>

      {rules.length === 0 ? (
        <p className="mb-3 text-sm text-gray-500">No return rules yet.</p>
      ) : (
        <div className="mb-3 space-y-2">
          {rules.map((rule) => (
            <RuleRow
              key={rule.id}
              rule={rule}
              buyerStages={sortedBuyer}
              publisherStages={sortedPublisher}
              usedFrom={usedFrom}
              onUpdate={onUpdate}
              onDelete={onDelete}
            />
          ))}
        </div>
      )}

      {availableFrom.length > 0 && (
        <div className="flex flex-wrap items-end gap-2 border-t border-gray-100 pt-3">
          <div className="min-w-[140px] flex-1">
            <div className="mb-1 text-xs font-semibold text-gray-500">From Stage</div>
            <Select value={addFrom || availableFrom[0]?.id || 0} onChange={(e) => setAddFrom(Number(e.target.value))}>
              {availableFrom.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </Select>
          </div>
          <div className="min-w-[140px] flex-1">
            <div className="mb-1 text-xs font-semibold text-gray-500">To Stage</div>
            <Select
              value={addTo || defaultReturnStageId || sortedPublisher[0]?.id || 0}
              onChange={(e) => setAddTo(Number(e.target.value))}
            >
              {sortedPublisher.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </Select>
          </div>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              const from = addFrom || availableFrom[0]?.id;
              const to = addTo || defaultReturnStageId || sortedPublisher[0]?.id;
              if (!from || !to) return;
              onAdd(from, to);
              setAddFrom(0);
              setAddTo(defaultReturnStageId);
            }}
          >
            <Plus className="h-4 w-4" /> Add
          </Button>
        </div>
      )}
    </div>
  );
}

function RuleRow({
  rule,
  buyerStages,
  publisherStages,
  usedFrom,
  onUpdate,
  onDelete,
}: {
  rule: ReturnRule;
  buyerStages: Stage[];
  publisherStages: Stage[];
  usedFrom: Set<number>;
  onUpdate: (ruleId: number, buyerStageId: number, returnStageId: number) => void;
  onDelete: (ruleId: number) => void;
}) {
  const fromOptions = buyerStages.filter((s) => s.id === rule.buyer_stage_id || !usedFrom.has(s.id));

  return (
    <div className="flex flex-wrap items-end gap-2 rounded-md border border-gray-100 px-3 py-2">
      <div className="min-w-[120px] flex-1">
        <div className="mb-1 text-xs font-semibold text-gray-500">From Stage</div>
        <Select
          value={rule.buyer_stage_id}
          onChange={(e) => onUpdate(rule.id, Number(e.target.value), rule.return_stage_id)}
        >
          {fromOptions.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </Select>
      </div>
      <div className="min-w-[120px] flex-1">
        <div className="mb-1 text-xs font-semibold text-gray-500">To Stage</div>
        <Select
          value={rule.return_stage_id}
          onChange={(e) => onUpdate(rule.id, rule.buyer_stage_id, Number(e.target.value))}
        >
          {publisherStages.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </Select>
      </div>
      <IconButton variant="danger" aria-label="Delete rule" onClick={() => onDelete(rule.id)}>
        <Trash2 className="h-4 w-4" />
      </IconButton>
    </div>
  );
}
