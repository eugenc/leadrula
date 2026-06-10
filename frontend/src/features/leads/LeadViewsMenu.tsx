import { useState } from "react";
import { ChevronDown, LayoutList, Pencil, Plus, Save, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Dropdown, DropdownItem } from "@/components/ui/dropdown";
import { Input, Label } from "@/components/ui/input";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { LeadFilterBuilder } from "./LeadFilterBuilder";
import {
  useSavedLeadViews,
  useCreateLeadView,
  useUpdateLeadView,
  useDeleteLeadView,
  useActiveViewId,
  mergeViews,
  getViewById,
  viewStateEqual,
  type FilterCondition,
  type SavedLeadView,
  type ViewPlacement,
} from "./leadsViews";

interface Props {
  placement: ViewPlacement;
  filters: FilterCondition[];
  onFiltersChange: (filters: FilterCondition[]) => void;
  columns?: string[];
  sort?: string;
  sortDir?: "asc" | "desc";
  onViewApply?: (view: SavedLeadView) => void;
}

export function LeadViewsMenu({
  placement,
  filters,
  onFiltersChange,
  columns = [],
  sort = "created_at",
  sortDir = "desc",
  onViewApply,
}: Props) {
  const isAdmin = useAuthStore((s) => s.user?.role === "admin");
  const [open, setOpen] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editView, setEditView] = useState<SavedLeadView | null>(null);
  const [saveAsNew, setSaveAsNew] = useState(false);
  const [name, setName] = useState("");
  const [shared, setShared] = useState(false);
  const [draftFilters, setDraftFilters] = useState<FilterCondition[]>([]);

  const { data: apiViews, isLoading } = useSavedLeadViews(placement);
  const views = mergeViews(apiViews, placement);
  const { activeId, setActiveId } = useActiveViewId(placement);
  const activeView = getViewById(views, activeId);

  const createView = useCreateLeadView();
  const updateView = useUpdateLeadView();
  const deleteView = useDeleteLeadView();

  const viewChanged = !viewStateEqual(activeView, {
    filters,
    columns,
    sort,
    sort_dir: sortDir,
  });

  async function switchView(view: SavedLeadView) {
    onViewApply?.(view);
    try {
      await setActiveId(view.public_id);
    } catch (err) {
      toast.error(errorMessage(err));
    }
    setOpen(false);
  }

  function openCreate() {
    setEditView(null);
    setSaveAsNew(true);
    setName("");
    setShared(false);
    setDraftFilters(filters.length ? [...filters] : [{ field: "assigned_user_id", op: "equals", value: "me" }]);
    setDialogOpen(true);
    setOpen(false);
  }

  function openEdit(view: SavedLeadView) {
    setEditView(view);
    setSaveAsNew(false);
    setName(view.name);
    setShared(view.shared);
    setDraftFilters([...view.filters]);
    setDialogOpen(true);
    setOpen(false);
  }

  function openSave(asNew: boolean) {
    setEditView(asNew || activeView.is_builtin ? null : activeView);
    setSaveAsNew(asNew || activeView.is_builtin);
    setName(asNew || activeView.is_builtin ? "" : activeView.name);
    setShared(false);
    setDraftFilters([...filters]);
    setDialogOpen(true);
    setOpen(false);
  }

  async function handleSave() {
    const trimmed = name.trim();
    if (!trimmed) {
      toast.error("Enter a view name");
      return;
    }
    const body = {
      name: trimmed,
      placement: placement === "list" ? "list" : "board",
      shared: isAdmin && shared,
      filters: draftFilters,
      ...(placement === "list" || placement === "board"
        ? { columns, sort, sort_dir: sortDir }
        : { columns }),
    };
    try {
      if (editView && !saveAsNew) {
        await updateView.mutateAsync({ id: editView.public_id, body });
        toast.success("View updated");
      } else {
        const created = await createView.mutateAsync(body);
        await setActiveId(created.public_id);
        onFiltersChange(created.filters);
        onViewApply?.(created);
        toast.success("View saved");
      }
      setDialogOpen(false);
    } catch (err) {
      toast.error(errorMessage(err));
    }
  }

  async function handleDelete(view: SavedLeadView) {
    try {
      await deleteView.mutateAsync(view.public_id);
      if (activeId === view.public_id) {
        const all = views.find((v) => v.public_id === "all");
        if (all) switchView(all);
      }
      toast.success("View deleted");
      setOpen(false);
    } catch (err) {
      toast.error(errorMessage(err));
    }
  }

  const pending = createView.isPending || updateView.isPending;

  return (
    <>
      <Dropdown
        open={open}
        onClose={() => setOpen(false)}
        align="left"
        className="max-h-96 w-60 overflow-y-auto"
        trigger={
          <Button variant="outline" size="sm" onClick={() => setOpen((o) => !o)} disabled={isLoading}>
            <LayoutList className="h-4 w-4" />
            {activeView.name}
            <ChevronDown className="h-4 w-4" />
          </Button>
        }
      >
        {views.map((v) => (
          <DropdownItem key={v.public_id} selected={v.public_id === activeId} onClick={() => switchView(v)}>
            {v.name}
            {v.shared && <span className="ml-1 text-xs text-gray-400">(shared)</span>}
          </DropdownItem>
        ))}
        <div className="my-1 border-t border-gray-100" />
        <DropdownItem onClick={openCreate}>
          <Plus className="mr-2 inline h-4 w-4" />
          Create view
        </DropdownItem>
        {viewChanged && (
          <>
            {!activeView.is_builtin && (
              <DropdownItem onClick={() => openSave(false)}>
                <Save className="mr-2 inline h-4 w-4" />
                Save view
              </DropdownItem>
            )}
            <DropdownItem onClick={() => openSave(true)}>
              <Save className="mr-2 inline h-4 w-4" />
              Save as new
            </DropdownItem>
          </>
        )}
        {views
          .filter((v) => !v.is_builtin && (!v.shared || isAdmin))
          .map((v) => (
            <div key={`actions-${v.public_id}`} className="flex">
              <DropdownItem className="flex-1" onClick={() => openEdit(v)}>
                <Pencil className="mr-2 inline h-3.5 w-3.5" />
                Edit "{v.name}"
              </DropdownItem>
            </div>
          ))}
        {views
          .filter((v) => !v.is_builtin && (!v.shared || isAdmin))
          .map((v) => (
            <DropdownItem
              key={`del-${v.public_id}`}
              className="text-danger hover:bg-danger-bg"
              onClick={() => handleDelete(v)}
            >
              <Trash2 className="mr-2 inline h-4 w-4" />
              Delete "{v.name}"
            </DropdownItem>
          ))}
      </Dropdown>

      <Dialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        title={saveAsNew ? "Save view" : editView ? "Edit view" : "Save view"}
        subtitle="All conditions must match (AND)."
        footer={
          <>
            <Button variant="secondary" onClick={() => setDialogOpen(false)}>
              Cancel
            </Button>
            <Button disabled={pending || !name.trim()} onClick={handleSave}>
              Save
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-4">
          <div>
            <Label>View name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="My view" className="mt-1.5" autoFocus />
          </div>
          {isAdmin && saveAsNew && (
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={shared} onChange={(e) => setShared(e.target.checked)} />
              Share with entire account
            </label>
          )}
          <LeadFilterBuilder conditions={draftFilters} onChange={setDraftFilters} />
        </div>
      </Dialog>
    </>
  );
}
