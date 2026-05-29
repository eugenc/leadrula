import {
  useMyContract,
  useReturnRules,
  useAddReturnRule,
  useDeleteReturnRule,
} from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { Card, Spinner, Badge } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/misc";
import { formatMoney } from "@/lib/utils";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";

export function ContractPage() {
  const { data: contract, isLoading } = useMyContract();
  const { data: stages } = useStages(contract?.buyer_pipeline_id);
  const { data: rules } = useReturnRules(null, true);
  const add = useAddReturnRule(true);
  const remove = useDeleteReturnRule(true);

  if (isLoading) return <Spinner className="h-6 w-6" />;
  if (!contract) return <p className="text-sm text-pd-muted">No contract assigned yet.</p>;

  const ruleStageIds = new Set((rules ?? []).map((r) => r.buyer_stage_id));
  const ruleByStage = new Map((rules ?? []).map((r) => [r.buyer_stage_id, r.id]));

  function toggle(stageId: number, on: boolean) {
    if (on) {
      add.mutate(
        { contractId: null, buyerStageId: stageId },
        { onError: (e) => toast.error(apiError(e).message) }
      );
    } else {
      const ruleId = ruleByStage.get(stageId);
      if (ruleId) remove.mutate(ruleId, { onError: (e) => toast.error(apiError(e).message) });
    }
  }

  return (
    <div className="max-w-2xl space-y-5">
      <PageHeader title="Contract" subtitle="Your agreement with the publisher." />
      <Card className="p-5">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-lg font-bold">{contract.name}</div>
            <Badge variant={contract.status === "active" ? "green" : "muted"}>{contract.status}</Badge>
          </div>
          <div className="text-right">
            <div className="text-xs uppercase text-pd-muted">Rate per lead</div>
            <div className="text-2xl font-bold">{formatMoney(contract.rate_per_lead)}</div>
          </div>
        </div>
      </Card>

      <Card className="p-5">
        <div className="mb-1 text-sm font-bold">Return Rules</div>
        <p className="mb-3 text-xs text-pd-muted">
          When a lead enters one of these stages, it is automatically returned to the publisher (no charge to you on return).
        </p>
        <div className="space-y-2">
          {[...(stages ?? [])]
            .sort((a, b) => a.position - b.position)
            .map((s) => (
              <div key={s.id} className="flex items-center justify-between border-b border-pd-border pb-2 last:border-0">
                <span className="font-medium">{s.name}</span>
                <Switch checked={ruleStageIds.has(s.id)} onChange={(v) => toggle(s.id, v)} />
              </div>
            ))}
        </div>
      </Card>
    </div>
  );
}
