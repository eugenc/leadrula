import { useRef, useState } from "react";
import { FormDrawer } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, FilterSelect } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { useAuthStore } from "@/store/authStore";
import { useUIStore } from "@/store/uiStore";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { effectiveFieldFormat } from "@/features/admin/customFieldConstants";
import { AddressAutocomplete } from "./AddressAutocomplete";
import { TagsInput, type TagsInputHandle } from "./LeadTagsEditor";
import { useCreateLead, usePipelines, useStages, useUsers, useCustomFields, useCustomFieldFolders, useTagSuggestions } from "./hooks";
import { groupCustomFieldsByFolder } from "@/features/admin/customFieldLayout";
import { isContactFolder, resolveContactBuiltinOrder } from "./contactSection";
import type { CustomField } from "@/types";
import {
  fromNativeDatetimeLocal,
  inputModeForFormat,
  normalizeCustomDateValue,
  toNativeDateValue,
  toNativeDatetimeLocalValue,
} from "./customFieldDate";
import { DatetimeFieldInput } from "./DatetimeFieldInput";

const BUILTINS = [
  { key: "first_name", label: "First Name", required: true },
  { key: "last_name", label: "Last Name" },
  { key: "phone", label: "Phone" },
  { key: "email", label: "Email" },
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
  const { data: customFieldFolders } = useCustomFieldFolders();
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
    country: "",
    source: "",
  });
  const [addressPlaceId, setAddressPlaceId] = useState("");
  const [pipelineId, setPipelineId] = useState(0);
  const [stageId, setStageId] = useState(0);
  const [assigneeId, setAssigneeId] = useState(0);
  const [tags, setTags] = useState<string[]>([]);
  const tagsInputRef = useRef<TagsInputHandle>(null);
  const [customValues, setCustomValues] = useState<Record<string, string>>({});
  const { data: tagSuggestions } = useTagSuggestions();

  const { data: stages } = useStages(pipelineId || undefined);
  const activeUsers = (users ?? []).filter((u) => u.status === "active");
  const activeCustom = (customFields ?? []).filter((f) => f.is_active);
  const grouped = groupCustomFieldsByFolder(customFieldFolders ?? [], activeCustom);
  const contactGroup = grouped.folders.find((g) => isContactFolder(g.folder));
  const contactCustomFields = contactGroup?.fields ?? [];
  const contactBuiltinOrder = resolveContactBuiltinOrder(contactGroup?.folder.contact_builtin_order);
  const otherCustomFields = [
    ...grouped.folders.filter((g) => !isContactFolder(g.folder)).flatMap((g) => g.fields),
    ...grouped.unassigned,
  ];

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
      country: "",
      source: "",
    });
    setPipelineId(0);
    setStageId(0);
    setAssigneeId(0);
    setTags([]);
    setCustomValues({});
    setAddressPlaceId("");
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

    const finalTags = tagsInputRef.current?.commitPending() ?? tags;

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
      country: fields.country?.trim() || undefined,
      source: fields.source?.trim() || undefined,
    };
    if (addressPlaceId) body.address_place_id = addressPlaceId;
    if (pipelineId && stageId) {
      body.pipeline_id = pipelineId;
      body.stage_id = stageId;
    }
    if (isAdmin && assigneeId) body.assigned_user_id = assigneeId;
    if (finalTags.length) body.tags = finalTags;
    if (Object.keys(cv).length) body.custom_values = cv;

    try {
      const lead = await create.mutateAsync(body);
      toast.success("Lead created");
      reset();
      onClose();
      openDetail(lead.id);
    } catch (err) {
      toast.error(errorMessage(err));
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
            <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
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
          </div>
        </div>

        <div>
          <SectionLabel className="mb-2">Contact</SectionLabel>
          <div className="flex flex-col gap-2.5">
            {contactBuiltinOrder.map((key) => {
              if (key === "address") {
                return (
                  <AddressAutocomplete
                    key={key}
                    address={fields.address ?? ""}
                    city={fields.city ?? ""}
                    state={fields.state ?? ""}
                    zip={fields.zip ?? ""}
                    country={fields.country ?? ""}
                    onPlainChange={(next) => {
                      setFields((prev) => ({ ...prev, ...next }));
                      setAddressPlaceId("");
                    }}
                    onSelect={(validated) => {
                      setFields((prev) => ({ ...prev, ...validated }));
                      setAddressPlaceId(validated.address_place_id);
                    }}
                  />
                );
              }
              if (key === "tags") {
                return (
                  <div key={key}>
                    <Label>Tags</Label>
                    <TagsInput
                      ref={tagsInputRef}
                      tags={tags}
                      onChange={setTags}
                      suggestions={tagSuggestions}
                      listId="new-lead-tag-suggestions"
                      className="mt-1"
                    />
                  </div>
                );
              }
              const builtin = BUILTINS.find((b) => b.key === key);
              if (!builtin) return null;
              return (
                <div key={key}>
                  <Label>
                    {builtin.label}
                    {"required" in builtin && builtin.required && <span className="text-danger"> *</span>}
                  </Label>
                  <Input
                    value={fields[builtin.key] ?? ""}
                    onChange={(e) => setField(builtin.key, e.target.value)}
                  />
                </div>
              );
            })}
            {contactCustomFields.map((f) => (
              <CustomFieldInput
                key={f.id}
                field={f}
                value={customValues[String(f.id)] ?? ""}
                onChange={(v) => setCustomValues((c) => ({ ...c, [String(f.id)]: v }))}
              />
            ))}
          </div>
        </div>

        {otherCustomFields.length > 0 && (
          <div>
            <SectionLabel className="mb-2">Custom Fields</SectionLabel>
            <div className="flex flex-col gap-2.5">
              {otherCustomFields.map((f) => (
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
  const formatToken =
    field.type === "date" || field.type === "datetime"
      ? effectiveFieldFormat(field.type, field.format)
      : "";
  const inputMode =
    field.type === "date" || field.type === "datetime"
      ? inputModeForFormat(field.type, formatToken)
      : "text";

  function handleBlur(next: string) {
    if (field.type === "date" || field.type === "datetime") {
      if (inputMode === "datetime-local") {
        onChange(fromNativeDatetimeLocal(next, field.type, field.format));
        return;
      }
      onChange(normalizeCustomDateValue(next, field.type, field.format));
      return;
    }
    onChange(next);
  }

  const displayValue =
    inputMode === "date"
      ? toNativeDateValue(value, field.type, field.format)
      : inputMode === "datetime-local"
        ? toNativeDatetimeLocalValue(value, field.type, field.format)
        : value;

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
      ) : field.type === "date" || field.type === "datetime" ? (
        inputMode === "datetime-local" ? (
          <DatetimeFieldInput
            value={displayValue}
            onChange={(next) => handleBlur(next)}
            className="w-full text-sm"
          />
        ) : (
          <Input
            value={displayValue}
            type={inputMode}
            placeholder={inputMode === "text" ? formatToken : undefined}
            onChange={(e) => {
              const next = e.target.value;
              if (inputMode === "text") {
                onChange(next);
              } else {
                handleBlur(next);
              }
            }}
            onBlur={(e) => {
              if (inputMode === "text") handleBlur(e.target.value);
            }}
            className="w-full"
          />
        )
      ) : (
        <Input
          value={value}
          type={field.type === "number" ? "number" : "text"}
          onChange={(e) => onChange(e.target.value)}
          className="w-full"
        />
      )}
    </div>
  );
}
