import { useState } from "react";
import { Select } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/layout/IconButton";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { Plus, Trash2 } from "lucide-react";
import type { ReturnRule, Stage } from "@/types";

type Props = {
  side: "buyer" | "publisher";
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

const BUYER_DESCRIPTION =
  "Pick which stages on your pipeline send leads back to the publisher. Return destination is set by the publisher on the offer.";

const PUBLISHER_DESCRIPTION =
  "When a lead enters the return start stage on the buyer pipeline, it moves to the publisher return destination stage.";

function sortStages(stages: Stage[]) {
  return [...stages].sort((a, b) => a.position - b.position);
}

export function ContractReturnRulesEditor({
  side,
  buyerStages,
  publisherStages,
  rules,
  defaultReturnStageId,
  loading,
  description,
  onAdd,
  onUpdate,
  onDelete,
}: Props) {
  const [draftOpen, setDraftOpen] = useState(false);
  const [addFrom, setAddFrom] = useState(0);
  const [addTo, setAddTo] = useState(defaultReturnStageId);

  const sortedBuyer = sortStages(buyerStages);
  const sortedPublisher = sortStages(publisherStages);
  const usedFrom = new Set(rules.map((r) => r.buyer_stage_id));
  const resolvedDescription =
    description ?? (side === "buyer" ? BUYER_DESCRIPTION : PUBLISHER_DESCRIPTION);

  if (loading) {
    return <Spinner className="h-5 w-5" />;
  }

  if (side === "buyer" && sortedBuyer.length === 0) {
    return <EmptyState title="Pipeline stages are required to configure return routes." />;
  }
  if (side === "publisher" && sortedPublisher.length === 0) {
    return <EmptyState title="Pipeline stages are required to configure return routes." />;
  }

  const availableFrom = sortedBuyer.filter((s) => !usedFrom.has(s.id));

  function closeDraft() {
    setDraftOpen(false);
    setAddFrom(0);
    setAddTo(defaultReturnStageId);
  }

  function openDraft() {
    if (side === "buyer") {
      setAddFrom(availableFrom[0]?.id ?? 0);
    } else {
      setAddTo(defaultReturnStageId || sortedPublisher[0]?.id || 0);
    }
    setDraftOpen(true);
  }

  function saveDraft() {
    if (side === "buyer") {
      const from = addFrom || availableFrom[0]?.id;
      if (!from) return;
      onAdd(from, defaultReturnStageId);
    } else {
      const to = addTo || defaultReturnStageId || sortedPublisher[0]?.id;
      const from = addFrom || availableFrom[0]?.id;
      if (!from || !to) return;
      onAdd(from, to);
    }
    closeDraft();
  }

  const canAdd = side === "buyer" ? availableFrom.length > 0 : availableFrom.length > 0;

  return (
    <div>
      <p className="mb-3 text-xs text-gray-400">{resolvedDescription}</p>

      {rules.length === 0 && !draftOpen && (
        <p className="mb-3 text-sm text-gray-500">No return routes yet.</p>
      )}

      {(rules.length > 0 || draftOpen) && (
        <div className="mb-3 space-y-2">
          {rules.map((rule) => (
            <RuleRow
              key={rule.id}
              side={side}
              rule={rule}
              buyerStages={sortedBuyer}
              publisherStages={sortedPublisher}
              usedFrom={usedFrom}
              onUpdate={onUpdate}
              onDelete={onDelete}
            />
          ))}
          {draftOpen && side === "buyer" && (
            <div className="flex flex-wrap items-end gap-2 rounded-md border border-gray-100 px-3 py-2">
              <div className="min-w-[120px] flex-1">
                <div className="mb-1 text-xs font-semibold text-gray-500">Return start</div>
                <Select
                  value={addFrom || availableFrom[0]?.id || 0}
                  onChange={(e) => setAddFrom(Number(e.target.value))}
                >
                  {availableFrom.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </Select>
              </div>
              <Button size="sm" variant="secondary" onClick={saveDraft}>
                <Plus className="h-4 w-4" /> Add
              </Button>
              <IconButton variant="danger" aria-label="Cancel new rule" onClick={closeDraft}>
                <Trash2 className="h-4 w-4" />
              </IconButton>
            </div>
          )}
          {draftOpen && side === "publisher" && (
            <div className="flex flex-wrap items-end gap-2 rounded-md border border-gray-100 px-3 py-2">
              <div className="min-w-[120px] flex-1">
                <div className="mb-1 text-xs font-semibold text-gray-500">Return start</div>
                <Select
                  value={addFrom || availableFrom[0]?.id || 0}
                  onChange={(e) => setAddFrom(Number(e.target.value))}
                >
                  {availableFrom.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </Select>
              </div>
              <div className="min-w-[120px] flex-1">
                <div className="mb-1 text-xs font-semibold text-gray-500">Return destination</div>
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
              <Button size="sm" variant="secondary" onClick={saveDraft}>
                <Plus className="h-4 w-4" /> Add
              </Button>
              <IconButton variant="danger" aria-label="Cancel new rule" onClick={closeDraft}>
                <Trash2 className="h-4 w-4" />
              </IconButton>
            </div>
          )}
        </div>
      )}

      {canAdd && !draftOpen && (
        <Button size="sm" variant="secondary" onClick={openDraft}>
          <Plus className="h-4 w-4" /> Add
        </Button>
      )}
    </div>
  );
}

function RuleRow({
  side,
  rule,
  buyerStages,
  publisherStages,
  usedFrom,
  onUpdate,
  onDelete,
}: {
  side: "buyer" | "publisher";
  rule: ReturnRule;
  buyerStages: Stage[];
  publisherStages: Stage[];
  usedFrom: Set<number>;
  onUpdate: (ruleId: number, buyerStageId: number, returnStageId: number) => void;
  onDelete: (ruleId: number) => void;
}) {
  const fromOptions = buyerStages.filter((s) => s.id === rule.buyer_stage_id || !usedFrom.has(s.id));

  if (side === "buyer") {
    return (
      <div className="flex flex-wrap items-end gap-2 rounded-md border border-gray-100 px-3 py-2">
        <div className="min-w-[120px] flex-1">
          <div className="mb-1 text-xs font-semibold text-gray-500">Return start</div>
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
        <IconButton variant="danger" aria-label="Delete rule" onClick={() => onDelete(rule.id)}>
          <Trash2 className="h-4 w-4" />
        </IconButton>
      </div>
    );
  }

  return (
    <div className="flex flex-wrap items-end gap-2 rounded-md border border-gray-100 px-3 py-2">
      <div className="min-w-[120px] flex-1">
        <div className="mb-1 text-xs font-semibold text-gray-500">Return destination</div>
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
