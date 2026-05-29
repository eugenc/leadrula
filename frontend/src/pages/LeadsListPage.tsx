import { useState } from "react";
import { useLeads, usePipelines } from "@/features/leads/hooks";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Badge, Spinner, EmptyState } from "@/components/ui/misc";
import { Select } from "@/components/ui/input";
import { PageHeader } from "@/components/layout/PageHeader";
import { useUIStore } from "@/store/uiStore";
import { format } from "date-fns";

const statusVariant: Record<string, "green" | "amber" | "muted" | "blue"> = {
  distributed: "green",
  returned: "amber",
  review: "blue",
  closed: "muted",
};

export function LeadsListPage() {
  const [status, setStatus] = useState("");
  const [pipelineId, setPipelineId] = useState(0);
  const { data: pipelines } = usePipelines();
  const { data: leads, isLoading } = useLeads({ status, pipeline_id: pipelineId });
  const openDetail = useUIStore((s) => s.openDetail);

  return (
    <div>
      <PageHeader title="Leads" subtitle="All leads visible to you." />
      <div className="mb-4 flex gap-3">
        <Select value={status} onChange={(e) => setStatus(e.target.value)} className="w-40">
          <option value="">All statuses</option>
          <option value="distributed">Distributed</option>
          <option value="returned">Returned</option>
          <option value="review">In review</option>
          <option value="closed">Closed</option>
        </Select>
        <Select value={pipelineId} onChange={(e) => setPipelineId(Number(e.target.value))} className="w-48">
          <option value={0}>All pipelines</option>
          {(pipelines ?? []).map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </Select>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <Spinner className="h-6 w-6" />
        </div>
      ) : (leads ?? []).length === 0 ? (
        <EmptyState title="No leads match these filters." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Name</TH>
              <TH>Phone</TH>
              <TH>Campaign</TH>
              <TH>Status</TH>
              <TH>Action At</TH>
              <TH>Created</TH>
            </tr>
          </THead>
          <TBody>
            {(leads ?? []).map((l) => (
              <TR key={l.id} onClick={() => openDetail(l.id)}>
                <TD className="font-semibold">
                  {l.first_name} {l.last_name}
                </TD>
                <TD>{l.phone ?? "—"}</TD>
                <TD>{l.campaign_name ?? "—"}</TD>
                <TD>
                  <Badge variant={statusVariant[l.status] ?? "muted"}>{l.status}</Badge>
                </TD>
                <TD>{l.action_at ? format(new Date(l.action_at), "MMM d, h:mma") : "—"}</TD>
                <TD>{format(new Date(l.created_at), "MMM d, yyyy")}</TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}
    </div>
  );
}
