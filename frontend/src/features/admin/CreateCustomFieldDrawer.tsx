import { useEffect, useState } from "react";
import { FormDrawer } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { CustomField } from "@/types";
import {
  CUSTOM_FIELD_TYPES,
  defaultFormatForType,
  formatPresetsForType,
  slugFieldKey,
} from "./customFieldConstants";

type FieldForm = { name: string; field_key: string; type: string; format: string; options: string };

function buildBody(form: FieldForm): Record<string, unknown> {
  const body: Record<string, unknown> = {
    name: form.name,
    field_key: form.field_key,
    type: form.type,
  };
  if (form.type === "dropdown") {
    body.options = form.options.split(",").map((s) => s.trim()).filter(Boolean);
  }
  if (form.type === "date" || form.type === "datetime") {
    body.format = form.format;
  }
  return body;
}

export function CreateCustomFieldDrawer({
  open,
  onClose,
  defaultName = "",
  defaultFieldKey = "",
  subtitle,
  onSubmit,
  isPending = false,
}: {
  open: boolean;
  onClose: () => void;
  defaultName?: string;
  defaultFieldKey?: string;
  subtitle?: string;
  onSubmit: (body: Record<string, unknown>) => Promise<CustomField>;
  isPending?: boolean;
}) {
  const [form, setForm] = useState<FieldForm>({
    name: "",
    field_key: "",
    type: "text",
    format: defaultFormatForType("text"),
    options: "",
  });
  const [fieldKeyTouched, setFieldKeyTouched] = useState(false);

  useEffect(() => {
    if (!open) return;
    setForm({
      name: defaultName,
      field_key: defaultFieldKey || slugFieldKey(defaultName),
      type: "text",
      format: defaultFormatForType("text"),
      options: "",
    });
    setFieldKeyTouched(false);
  }, [open, defaultName, defaultFieldKey]);

  const canSubmit = !!form.name && !!form.field_key;

  async function submit() {
    try {
      const field = await onSubmit(buildBody(form));
      toast.success(`Custom field "${field.name}" created`);
      onClose();
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  return (
    <FormDrawer
      open={open}
      onClose={onClose}
      title="Create custom field"
      subtitle={subtitle}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={!canSubmit || isPending}>
            Create
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div>
          <Label>Name</Label>
          <Input
            value={form.name}
            onChange={(e) => {
              const name = e.target.value;
              setForm((f) => ({
                ...f,
                name,
                field_key: fieldKeyTouched ? f.field_key : slugFieldKey(name),
              }));
            }}
          />
        </div>
        <div>
          <Label>Field Key</Label>
          <Input
            value={form.field_key}
            onChange={(e) => {
              setFieldKeyTouched(true);
              setForm({ ...form, field_key: e.target.value });
            }}
            placeholder="utility_provider"
          />
        </div>
        <div>
          <Label>Type</Label>
          <Select
            value={form.type}
            onChange={(e) => {
              const type = e.target.value;
              setForm((f) => ({
                ...f,
                type,
                format: defaultFormatForType(type),
              }));
            }}
          >
            {CUSTOM_FIELD_TYPES.map(({ value, label }) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </Select>
        </div>
        {(form.type === "date" || form.type === "datetime") && (
          <div>
            <Label>Format</Label>
            <Select value={form.format} onChange={(e) => setForm({ ...form, format: e.target.value })}>
              {formatPresetsForType(form.type).map(({ value, label }) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </Select>
          </div>
        )}
        {form.type === "dropdown" && (
          <div>
            <Label>Options (comma separated)</Label>
            <Input value={form.options} onChange={(e) => setForm({ ...form, options: e.target.value })} />
          </div>
        )}
      </div>
    </FormDrawer>
  );
}
