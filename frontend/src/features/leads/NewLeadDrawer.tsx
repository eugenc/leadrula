import { useState } from "react";
import { FormDrawer } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, FilterSelect } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { useAuthStore } from "@/store/authStore";
import { useUIStore } from "@/store/uiStore";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { useCreateLead, usePipelines, useStages, useUsers, useCustomFields } from "./hooks";
import type { CustomField } from "@/types";

const BUILTINS = [
  { key: "first_name", label: "First Name", required: true },
  { key: "last_name", label: "Last Name" },
  { key: "phone", label: "Phone" },
  { key: "email", label: "Email" },
  { key: "address", label: "Address" },
  { key: "city", label: "City" },
  { key: "state", label: "State" },
  { key: "zip", label: "Zip" },
] as const;

interface Props {
  open: boolean;
  onClose: () => void;
}

export function NewLeadDrawer({ open, onClose }: Props) {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const openDetail = useUIStore((s) => s.openDetail);
  const create = useCreateLead();

  const { data: pipelines } = usePipelines();
  const { data: customFields } = useCustomFields();
  const { data: users } = useUsers();

  const [fields, setFields] = useState<Record<string, string>>({
    first_name: "",
    last_name: "",
    phone: "",
    email: "",
    address: "",
    city: "",
    state: "",
    zip: "",
    source: "",
  });
  const [pipelineId, setPipelineId] = useState(0);
  const [stageId, setStageId] = useState(0);
  const [assigneeId, setAssigneeId] = useState(0);
  const [tagsInput, setTagsInput] = useState("");
  const [customValues, setCustomValues] = useState<Record<string, string>>({});

  const { data: stages } = useStages(pipelineId || undefined);
  const activeUsers = (users ?? []).filter((u) => u.status === "active");
  const activeCustom = (customFields ?? []).filter((f) => f.is_active);

  function setField(key: string, val: string) {
    setFields((f) => ({ ...f, [key]: val }));
  }

  function reset() {
    setFields({
      first_name: "",
      last_name: "",
      phone: "",
      email: "",
      address: "",
      city: "",
      state: "",
      zip: "",
      source: "",
    });
    setPipelineId(0);
    setStageId(0);
    setAssigneeId(0);
    setTagsInput("");
    setCustomValues({});
  }

  async function handleSubmit() {
    const first = fields.first_name?.trim() ?? "";
    const phone = fields.phone?.trim() ?? "";
    const email = fields.email?.trim() ?? "";
    if (!first) {
      toast.error("First name is required");
      return;
    }
    if (!phone && !email) {
      toast.error("Phone or email is required");
      return;
    }

    const tags = tagsInput
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean);

    const cv: Record<string, string> = {};
    for (const [k, v] of Object.entries(customValues)) {
      if (v.trim()) cv[k] = v.trim();
    }

    const body: Record<string, unknown> = {
      first_name: first,
      last_name: fields.last_name?.trim() || undefined,
      phone: phone || undefined,
      email: email || undefined,
      address: fields.address?.trim() || undefined,
      city: fields.city?.trim() || undefined,
      state: fields.state?.trim() || undefined,
      zip: fields.zip?.trim() || undefined,
      source: fields.source?.trim() || undefined,
    };
    if (pipelineId && stageId) {
      body.pipeline_id = pipelineId;
      body.stage_id = stageId;
    }
    if (isAdmin && assigneeId) body.assigned_user_id = assigneeId;
    if (tags.length) body.tags = tags;
    if (Object.keys(cv).length) body.custom_values = cv;

    try {
      const lead = await create.mutateAsync(body);
      toast.success("Lead created");
      reset();
      onClose();
      openDetail(lead.id);
    } catch (err) {
      toast.error(apiError(err).message);
    }
  }

  return (
    <FormDrawer
      open={open}
      onClose={onClose}
      title="New Lead"
      subtitle="Fill in lead details before saving."
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={create.isPending} onClick={handleSubmit}>
            Create lead
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <div>
          <SectionLabel className="mb-2">Lead</SectionLabel>
          <div className="flex flex-col gap-2.5">
            <div className="grid grid-cols-2 gap-2.5">
              <div>
                <Label>Pipeline</Label>
                <FilterSelect
                  value={pipelineId}
                  onChange={(e) => {
                    setPipelineId(Number(e.target.value));
                    setStageId(0);
                  }}
                  className="w-full"
                >
                  <option value={0}>None</option>
                  {(pipelines ?? []).map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </FilterSelect>
              </div>
              {pipelineId > 0 && (
                <div>
                  <Label>Stage</Label>
                  <FilterSelect
                    value={stageId}
                    onChange={(e) => setStageId(Number(e.target.value))}
                  className="w-full"
                >
                  <option value={0}>Select stage…</option>
                    {(stages ?? []).map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </FilterSelect>
                </div>
              )}
            </div>
            {isAdmin && (
              <div>
                <Label>Assigned To</Label>
                <FilterSelect
                  value={assigneeId}
                  onChange={(e) => setAssigneeId(Number(e.target.value))}
                  className="w-full"
                >
                  <option value={0}>Unassigned</option>
                  {activeUsers.map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.full_name}
                    </option>
                  ))}
                </FilterSelect>
              </div>
            )}
            <div>
              <Label>Source</Label>
              <Input
                value={fields.source ?? ""}
                onChange={(e) => setField("source", e.target.value)}
              />
            </div>
            <div>
              <Label>Tags</Label>
              <Input
                value={tagsInput}
                onChange={(e) => setTagsInput(e.target.value)}
                placeholder="Comma-separated tags"
              />
            </div>
          </div>
        </div>

        <div>
          <SectionLabel className="mb-2">Contact</SectionLabel>
          <div className="flex flex-col gap-2.5">
            {BUILTINS.map((b) => (
              <div key={b.key}>
                <Label>
                  {b.label}
                  {"required" in b && b.required && <span className="text-danger"> *</span>}
                </Label>
                <Input
                  value={fields[b.key] ?? ""}
                  onChange={(e) => setField(b.key, e.target.value)}
                />
              </div>
            ))}
          </div>
        </div>

        {activeCustom.length > 0 && (
          <div>
            <SectionLabel className="mb-2">Custom Fields</SectionLabel>
            <div className="flex flex-col gap-2.5">
              {activeCustom.map((f) => (
                <CustomFieldInput
                  key={f.id}
                  field={f}
                  value={customValues[String(f.id)] ?? ""}
                  onChange={(v) => setCustomValues((c) => ({ ...c, [String(f.id)]: v }))}
                />
              ))}
            </div>
          </div>
        )}
      </div>
    </FormDrawer>
  );
}

function CustomFieldInput({
  field,
  value,
  onChange,
}: {
  field: CustomField;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <div>
      <Label>{field.name}</Label>
      {field.type === "dropdown" && field.options?.length ? (
        <Select value={value} onChange={(e) => onChange(e.target.value)} className="w-full">
          <option value="">—</option>
          {field.options.map((o) => (
            <option key={o} value={o}>
              {o}
            </option>
          ))}
        </Select>
      ) : field.type === "checkbox" ? (
        <Select value={value} onChange={(e) => onChange(e.target.value)} className="w-full">
          <option value="">—</option>
          <option value="true">Yes</option>
          <option value="false">No</option>
        </Select>
      ) : (
        <Input value={value} onChange={(e) => onChange(e.target.value)} />
      )}
    </div>
  );
}
