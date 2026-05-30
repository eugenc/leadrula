import { Switch, Spinner, EmptyState } from "@/components/ui/misc";
import type { ReturnRule, Stage } from "@/types";

type Props = {
  stages: Stage[];
  rules: ReturnRule[];
  onToggle: (stageId: number, on: boolean) => void;
  loading?: boolean;
  description?: string;
};

const DEFAULT_DESCRIPTION =
  "When a lead enters one of these stages, it is automatically returned to the publisher.";

export function ContractReturnRulesEditor({
  stages,
  rules,
  onToggle,
  loading,
  description = DEFAULT_DESCRIPTION,
}: Props) {
  const ruleStageIds = new Set(rules.map((r) => r.buyer_stage_id));
  const sorted = [...stages].sort((a, b) => a.position - b.position);

  if (loading) {
    return <Spinner className="h-5 w-5" />;
  }

  if (sorted.length === 0) {
    return <EmptyState title="No stages in buyer pipeline." />;
  }

  return (
    <div>
      <p className="mb-3 text-xs text-gray-400">{description}</p>
      <div className="space-y-2">
        {sorted.map((s) => (
          <div
            key={s.id}
            className="flex items-center justify-between border-b border-gray-100 pb-2 last:border-0"
          >
            <span className="font-medium text-gray-800">{s.name}</span>
            <Switch checked={ruleStageIds.has(s.id)} onChange={(v) => onToggle(s.id, v)} />
          </div>
        ))}
      </div>
    </div>
  );
}
