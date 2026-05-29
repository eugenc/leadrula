import { useState } from "react";
import {
  useCampaigns,
  useCreateCampaign,
  useUpdateCampaign,
  useDeleteCampaign,
  useFieldMap,
  useAddFieldMap,
  useDeleteFieldMap,
  useBuyers,
  useBuyerPipelines,
  useBuyerStages,
} from "@/features/admin/hooks";
import { useCustomFields } from "@/features/leads/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Switch, Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { Dialog } from "@/components/ui/dialog";
import { Plus, Trash2 } from "lucide-react";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";

const BUILTINS = ["first_name", "last_name", "phone", "email", "address", "city", "state", "zip"];

export function RoutingPage() {
  const { data: campaigns, isLoading } = useCampaigns();
  const update = useUpdateCampaign();
  const remove = useDeleteCampaign();
  const [open, setOpen] = useState(false);
  const [mapFor, setMapFor] = useState<number | null>(null);

  return (
    <div>
      <PageHeader
        title="Routing"
        subtitle="Map inbound campaigns to buyer pipelines."
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> New Campaign
          </Button>
        }
      />
      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (campaigns ?? []).length === 0 ? (
        <EmptyState title="No routing campaigns yet." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Campaign</TH>
              <TH>Active</TH>
              <TH />
            </tr>
          </THead>
          <TBody>
            {(campaigns ?? []).map((c) => (
              <TR key={c.id}>
                <TD className="font-mono font-semibold">{c.campaign_name}</TD>
                <TD>
                  <Switch checked={c.is_active} onChange={(v) => update.mutate({ id: c.id, body: { is_active: v } })} />
                </TD>
                <TD>
                  <div className="flex justify-end gap-2">
                    <Button size="sm" variant="outline" onClick={() => setMapFor(c.id)}>
                      Field Map
                    </Button>
                    <button
                      onClick={() => remove.mutate(c.id, { onError: (e) => toast.error(apiError(e).message) })}
                      className="text-pd-muted hover:text-pd-red"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}
      {open && <CreateCampaignDialog onClose={() => setOpen(false)} />}
      {mapFor && <FieldMapDialog campaignId={mapFor} onClose={() => setMapFor(null)} />}
    </div>
  );
}

function CreateCampaignDialog({ onClose }: { onClose: () => void }) {
  const { data: buyers } = useBuyers();
  const create = useCreateCampaign();
  const [buyerId, setBuyerId] = useState(0);
  const [pipelineId, setPipelineId] = useState(0);
  const [stageId, setStageId] = useState(0);
  const [name, setName] = useState("");
  const { data: pipelines } = useBuyerPipelines(buyerId || null);
  const { data: stages } = useBuyerStages(buyerId || null, pipelineId || null);

  return (
    <Dialog open onClose={onClose} title="New Routing Campaign">
      <div className="space-y-3">
        <div>
          <Label>Campaign name (matches inbound payload)</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="solar_ontario_q2" />
        </div>
        <div>
          <Label>Buyer</Label>
          <Select value={buyerId} onChange={(e) => setBuyerId(Number(e.target.value))}>
            <option value={0}>Select…</option>
            {(buyers ?? []).map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </Select>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <Label>Target pipeline</Label>
            <Select value={pipelineId} onChange={(e) => setPipelineId(Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(pipelines ?? []).map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Target stage</Label>
            <Select value={stageId} onChange={(e) => setStageId(Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(stages ?? []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </Select>
          </div>
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!name || !pipelineId || !stageId}
            onClick={() =>
              create.mutate(
                { campaign_name: name, target_pipeline_id: pipelineId, target_stage_id: stageId },
                {
                  onSuccess: () => {
                    toast.success("Campaign created");
                    onClose();
                  },
                  onError: (e) => toast.error(apiError(e).message),
                }
              )
            }
          >
            Create
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

function FieldMapDialog({ campaignId, onClose }: { campaignId: number; onClose: () => void }) {
  const { data: entries } = useFieldMap(campaignId);
  const { data: customFields } = useCustomFields();
  const add = useAddFieldMap();
  const remove = useDeleteFieldMap();
  const [sourceKey, setSourceKey] = useState("");
  const [target, setTarget] = useState("first_name");

  function submit() {
    const isCustom = target.startsWith("cf:");
    const body: Record<string, unknown> = isCustom
      ? { source_key: sourceKey, target_type: "custom", custom_field_id: Number(target.slice(3)) }
      : { source_key: sourceKey, target_type: "builtin", builtin_field: target };
    add.mutate(
      { campaignId, body },
      {
        onSuccess: () => setSourceKey(""),
        onError: (e) => toast.error(apiError(e).message),
      }
    );
  }

  return (
    <Dialog open onClose={onClose} title="Field Mapping">
      <div className="space-y-3">
        <div className="space-y-1">
          {(entries ?? []).length === 0 && <p className="text-sm text-pd-muted">No mappings yet.</p>}
          {(entries ?? []).map((e) => (
            <div key={e.id} className="flex items-center justify-between rounded border border-pd-border px-3 py-2 text-sm">
              <span>
                <span className="font-mono">{e.source_key}</span> →{" "}
                {e.target_type === "builtin" ? (
                  <Badge variant="blue">{e.builtin_field}</Badge>
                ) : (
                  <Badge variant="green">custom #{e.custom_field_id}</Badge>
                )}
              </span>
              <button onClick={() => remove.mutate(e.id)} className="text-pd-muted hover:text-pd-red">
                <Trash2 className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
        <div className="grid grid-cols-[1fr_1fr_auto] items-end gap-2">
          <div>
            <Label>Source key</Label>
            <Input value={sourceKey} onChange={(e) => setSourceKey(e.target.value)} placeholder="phone_number" />
          </div>
          <div>
            <Label>Target</Label>
            <Select value={target} onChange={(e) => setTarget(e.target.value)}>
              <optgroup label="Built-in">
                {BUILTINS.map((b) => (
                  <option key={b} value={b}>
                    {b}
                  </option>
                ))}
              </optgroup>
              <optgroup label="Custom">
                {(customFields ?? []).map((f) => (
                  <option key={f.id} value={`cf:${f.id}`}>
                    {f.name}
                  </option>
                ))}
              </optgroup>
            </Select>
          </div>
          <Button onClick={submit} disabled={!sourceKey}>
            <Plus className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
