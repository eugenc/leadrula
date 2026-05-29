import { useState } from "react";
import { useDisqReasons } from "@/features/leads/hooks";
import { useCreateReason, useUpdateReason, useDeleteReason } from "@/features/admin/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { Card, Switch, Spinner } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Plus, Trash2 } from "lucide-react";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";

export function DisqReasonsPage() {
  const { data: reasons, isLoading } = useDisqReasons();
  const create = useCreateReason();
  const update = useUpdateReason();
  const remove = useDeleteReason();
  const [label, setLabel] = useState("");

  return (
    <div className="max-w-2xl">
      <PageHeader title="Disqualification Reasons" subtitle="Used when a stage prompts for a reason." />
      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (
        <Card className="p-4">
          <div className="space-y-2">
            {(reasons ?? []).map((r) => (
              <div key={r.id} className="flex items-center justify-between gap-3 border-b border-pd-border pb-2 last:border-0">
                <span className="font-medium">{r.label}</span>
                <div className="flex items-center gap-3">
                  <Switch checked={r.is_active} onChange={(v) => update.mutate({ id: r.id, body: { is_active: v } })} />
                  <button
                    onClick={() => remove.mutate(r.id, { onError: (e) => toast.error(apiError(e).message) })}
                    className="text-pd-muted hover:text-pd-red"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
            ))}
          </div>
          <div className="mt-4 flex gap-2">
            <Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="New reason" />
            <Button onClick={() => label && create.mutate(label, { onSuccess: () => setLabel("") })}>
              <Plus className="h-4 w-4" /> Add
            </Button>
          </div>
        </Card>
      )}
    </div>
  );
}
