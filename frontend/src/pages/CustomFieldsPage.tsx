import { useEffect, useState } from "react";
import { useCustomFields } from "@/features/leads/hooks";
import { useCreateField, useUpdateField, useDeleteField } from "@/features/admin/hooks";
import { ImportCustomFieldsModal } from "@/features/admin/ImportCustomFieldsModal";
import { slugFieldKey } from "@/features/admin/customFieldConstants";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Switch, Spinner } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { Plus, Trash2, Upload } from "lucide-react";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import type { CustomField } from "@/types";

const CUSTOM_FIELD_TYPES = [
  { value: "text", label: "Text" },
  { value: "number", label: "Number" },
  { value: "date", label: "Date" },
  { value: "datetime", label: "Date & time" },
  { value: "dropdown", label: "Dropdown" },
  { value: "checkbox", label: "Checkbox" },
] as const;

type FieldForm = { name: string; field_key: string; type: string; options: string };

const emptyForm = (): FieldForm => ({ name: "", field_key: "", type: "text", options: "" });

function fieldToForm(f: CustomField): FieldForm {
  return {
    name: f.name,
    field_key: f.field_key,
    type: f.type,
    options: f.type === "dropdown" ? (f.options ?? []).join(", ") : "",
  };
}

export function CustomFieldsPage() {
  const { data: fields, isLoading } = useCustomFields();
  const create = useCreateField();
  const update = useUpdateField();
  const remove = useDeleteField();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [editing, setEditing] = useState<CustomField | null>(null);
  const [form, setForm] = useState<FieldForm>(emptyForm());
  const [fieldKeyTouched, setFieldKeyTouched] = useState(false);

  useEffect(() => {
    if (!drawerOpen) return;
    setForm(editing ? fieldToForm(editing) : emptyForm());
    setFieldKeyTouched(false);
  }, [drawerOpen, editing]);

  function openCreate() {
    setEditing(null);
    setDrawerOpen(true);
  }

  function openEdit(field: CustomField) {
    setEditing(field);
    setDrawerOpen(true);
  }

  function closeDrawer() {
    setDrawerOpen(false);
    setEditing(null);
  }

  function buildBody(): Record<string, unknown> {
    const body: Record<string, unknown> = {
      name: form.name,
      field_key: form.field_key,
    };
    if (form.type === "dropdown") {
      body.options = form.options.split(",").map((s) => s.trim()).filter(Boolean);
    }
    return body;
  }

  function submit() {
    if (editing) {
      update.mutate(
        { id: editing.id, body: buildBody() },
        {
          onSuccess: () => {
            toast.success("Field updated");
            closeDrawer();
          },
          onError: (e) => toast.error(apiError(e).message),
        }
      );
    } else {
      create.mutate(
        { ...buildBody(), type: form.type },
        {
          onSuccess: () => {
            toast.success("Field created");
            closeDrawer();
          },
          onError: (e) => toast.error(apiError(e).message),
        }
      );
    }
  }

  const saving = create.isPending || update.isPending;
  const canSubmit = !!form.name && !!form.field_key;

  return (
    <>
      <PageHeader
        action={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => setImportOpen(true)}>
              <Upload className="h-4 w-4" /> Import CSV
            </Button>
            <Button onClick={openCreate}>
              <Plus className="h-4 w-4" /> New Field
            </Button>
          </div>
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
                <TR
                  key={f.id}
                  className="cursor-pointer hover:bg-gray-50"
                  onClick={() => openEdit(f)}
                >
                  <TD className="font-medium text-gray-800">{f.name}</TD>
                  <TD className="font-mono text-xs">{f.field_key}</TD>
                  <TD>{CUSTOM_FIELD_TYPES.find((t) => t.value === f.type)?.label ?? f.type}</TD>
                  <TD>
                    <div onClick={(e) => e.stopPropagation()}>
                      <Switch
                        checked={f.is_active}
                        onChange={(v) => update.mutate({ id: f.id, body: { is_active: v } })}
                      />
                    </div>
                  </TD>
                  <TD>
                    <div className="flex justify-end" onClick={(e) => e.stopPropagation()}>
                      <IconButton
                        variant="danger"
                        onClick={() => remove.mutate(f.id, { onError: (e) => toast.error(apiError(e).message) })}
                      >
                        <Trash2 className="h-4 w-4" />
                      </IconButton>
                    </div>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}

        <FormDrawer
          open={drawerOpen}
          onClose={closeDrawer}
          title={editing ? editing.name : "New Custom Field"}
          subtitle={editing ? "Edit custom field" : undefined}
          footer={
            <>
              <Button variant="secondary" onClick={closeDrawer}>
                Cancel
              </Button>
              <Button onClick={submit} disabled={!canSubmit || saving}>
                {editing ? "Save" : "Create"}
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
                    field_key: !editing && !fieldKeyTouched ? slugFieldKey(name) : f.field_key,
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
              {editing && editing.field_key !== form.field_key && (
                <p className="mt-1 text-xs text-gray-500">
                  Renaming the key updates pipeline stage rules that reference this field.
                </p>
              )}
            </div>
            <div>
              <Label>Type</Label>
              {editing ? (
                <p className="text-sm text-gray-700">
                  {CUSTOM_FIELD_TYPES.find((t) => t.value === editing.type)?.label ?? editing.type}
                </p>
              ) : (
                <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
                  {CUSTOM_FIELD_TYPES.map(({ value, label }) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </Select>
              )}
            </div>
            {(editing ? editing.type === "dropdown" : form.type === "dropdown") && (
              <div>
                <Label>Options (comma separated)</Label>
                <Input value={form.options} onChange={(e) => setForm({ ...form, options: e.target.value })} />
              </div>
            )}
          </div>
        </FormDrawer>
      </PageBody>

      <ImportCustomFieldsModal open={importOpen} onClose={() => setImportOpen(false)} />
    </>
  );
}
