import { useState } from "react";
import { useCustomFields } from "@/features/leads/hooks";
import { useCreateField, useUpdateField, useDeleteField } from "@/features/admin/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Switch, Spinner } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { Plus, Trash2 } from "lucide-react";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";

const CUSTOM_FIELD_TYPES = [
  { value: "text", label: "Text" },
  { value: "number", label: "Number" },
  { value: "date", label: "Date" },
  { value: "datetime", label: "Date & time" },
  { value: "dropdown", label: "Dropdown" },
  { value: "checkbox", label: "Checkbox" },
] as const;

export function CustomFieldsPage() {
  const { data: fields, isLoading } = useCustomFields();
  const create = useCreateField();
  const update = useUpdateField();
  const remove = useDeleteField();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({ name: "", field_key: "", type: "text", options: "" });

  function submit() {
    const body: Record<string, unknown> = {
      name: form.name,
      field_key: form.field_key,
      type: form.type,
    };
    if (form.type === "dropdown") body.options = form.options.split(",").map((s) => s.trim()).filter(Boolean);
    create.mutate(body, {
      onSuccess: () => {
        setOpen(false);
        setForm({ name: "", field_key: "", type: "text", options: "" });
      },
      onError: (e) => toast.error(apiError(e).message),
    });
  }

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> New Field
          </Button>
        }
      />
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (
          <Table>
            <THead>
              <tr>
                <TH>Name</TH>
                <TH>Key</TH>
                <TH>Type</TH>
                <TH>Active</TH>
                <TH />
              </tr>
            </THead>
            <TBody>
              {(fields ?? []).map((f) => (
                <TR key={f.id}>
                  <TD className="font-medium text-gray-800">{f.name}</TD>
                  <TD className="font-mono text-xs">{f.field_key}</TD>
                  <TD>{CUSTOM_FIELD_TYPES.find((t) => t.value === f.type)?.label ?? f.type}</TD>
                  <TD>
                    <Switch
                      checked={f.is_active}
                      onChange={(v) => update.mutate({ id: f.id, body: { is_active: v } })}
                    />
                  </TD>
                  <TD>
                    <IconButton
                      variant="danger"
                      onClick={() => remove.mutate(f.id, { onError: (e) => toast.error(apiError(e).message) })}
                    >
                      <Trash2 className="h-4 w-4" />
                    </IconButton>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}

        <FormDrawer
          open={open}
          onClose={() => setOpen(false)}
          title="New Custom Field"
          footer={
            <>
              <Button variant="secondary" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button onClick={submit} disabled={!form.name || !form.field_key}>
                Create
              </Button>
            </>
          }
        >
          <div className="space-y-3">
            <div>
              <Label>Name</Label>
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </div>
            <div>
              <Label>Field Key</Label>
              <Input
                value={form.field_key}
                onChange={(e) => setForm({ ...form, field_key: e.target.value })}
                placeholder="utility_provider"
              />
            </div>
            <div>
              <Label>Type</Label>
              <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
                {CUSTOM_FIELD_TYPES.map(({ value, label }) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </Select>
            </div>
            {form.type === "dropdown" && (
              <div>
                <Label>Options (comma separated)</Label>
                <Input value={form.options} onChange={(e) => setForm({ ...form, options: e.target.value })} />
              </div>
            )}
          </div>
        </FormDrawer>
      </PageBody>
    </>
  );
}
