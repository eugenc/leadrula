import { useEffect, useMemo, useState } from "react";
import {
  DragDropContext,
  Droppable,
  Draggable,
  type DropResult,
} from "@hello-pangea/dnd";
import { GripVertical, Plus, Trash2, FolderOpen } from "lucide-react";
import { useCustomFields, useCustomFieldFolders } from "@/features/leads/hooks";
import {
  useCreateCustomFieldFolder,
  useUpdateCustomFieldFolder,
  useDeleteCustomFieldFolder,
  useSaveCustomFieldLayout,
} from "@/features/admin/hooks";
import {
  applyDrag,
  buildLayoutPayload,
  folderDroppableId,
  groupCustomFieldsByFolder,
  UNASSIGNED_DROPPABLE,
  type GroupedFields,
} from "@/features/admin/customFieldLayout";
import { DeletePipelineResourceConfirmDialog } from "@/features/pipelines/DeletePipelineResourceConfirmDialog";
import { Card, Spinner, EmptyState } from "@/components/ui/misc";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { IconButton } from "@/components/layout/IconButton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { CustomField } from "@/types";

export function CustomFieldFoldersTab() {
  const { data: folders, isLoading: foldersLoading } = useCustomFieldFolders();
  const { data: fields, isLoading: fieldsLoading } = useCustomFields();
  const createFolder = useCreateCustomFieldFolder();
  const updateFolder = useUpdateCustomFieldFolder();
  const deleteFolder = useDeleteCustomFieldFolder();
  const saveLayout = useSaveCustomFieldLayout();

  const [newFolderName, setNewFolderName] = useState("");
  const [folderToDelete, setFolderToDelete] = useState<{ id: number; name: string } | null>(null);

  const serverGrouped = useMemo(
    () => groupCustomFieldsByFolder(folders ?? [], (fields ?? []).filter((f) => f.is_active)),
    [folders, fields]
  );
  // Optimistic copy so cross-folder drags don't snap back while the save is in flight.
  const [draft, setDraft] = useState<GroupedFields | null>(null);
  useEffect(() => setDraft(null), [serverGrouped]);
  const view = draft ?? serverGrouped;

  function onDragEnd(result: DropResult) {
    const next = applyDrag(view, result);
    if (!next) return;
    setDraft(next);
    saveLayout.mutate(buildLayoutPayload(next), {
      onError: (e) => {
        toast.error(errorMessage(e));
        setDraft(null);
      },
    });
  }

  function addFolder() {
    const name = newFolderName.trim();
    if (!name) return;
    createFolder.mutate(name, {
      onSuccess: () => setNewFolderName(""),
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  if (foldersLoading || fieldsLoading) return <Spinner className="h-6 w-6" />;

  const hasFields = view.folders.some((g) => g.fields.length) || view.unassigned.length > 0;

  return (
    <>
      <Card className="p-4">
        <div className="mb-4 flex gap-2">
          <Input
            value={newFolderName}
            onChange={(e) => setNewFolderName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && addFolder()}
            placeholder="New folder name"
            className="h-9 max-w-xs text-sm"
          />
          <Button onClick={addFolder} disabled={!newFolderName.trim() || createFolder.isPending}>
            <Plus className="h-4 w-4" /> Add Folder
          </Button>
        </div>

        <DragDropContext onDragEnd={onDragEnd}>
          <Droppable droppableId="folder-list" type="folder">
            {(provided) => (
              <div ref={provided.innerRef} {...provided.droppableProps} className="space-y-3">
                {view.folders.map((group, index) => (
                  <Draggable key={group.folder.id} draggableId={`folder-${group.folder.id}`} index={index}>
                    {(drag, snapshot) => (
                      <div
                        ref={drag.innerRef}
                        {...drag.draggableProps}
                        className={cn(
                          "rounded-lg border border-gray-200 bg-white",
                          snapshot.isDragging && "shadow-md"
                        )}
                      >
                        <div className="flex items-center gap-2 border-b border-gray-100 px-2 py-2">
                          <span
                            {...drag.dragHandleProps}
                            className="cursor-grab px-1 text-gray-400 hover:text-gray-600"
                          >
                            <GripVertical className="h-4 w-4" />
                          </span>
                          <FolderOpen className="h-4 w-4 shrink-0 text-gray-400" />
                          <Input
                            defaultValue={group.folder.name}
                            key={group.folder.name}
                            className="h-8 flex-1 text-sm font-medium"
                            onBlur={(e) => {
                              const name = e.target.value.trim();
                              if (!name) {
                                e.target.value = group.folder.name;
                                return;
                              }
                              if (name !== group.folder.name) {
                                updateFolder.mutate(
                                  { id: group.folder.id, body: { name } },
                                  { onError: (err) => toast.error(errorMessage(err)) }
                                );
                              }
                            }}
                          />
                          <IconButton
                            variant="danger"
                            onClick={() => setFolderToDelete({ id: group.folder.id, name: group.folder.name })}
                          >
                            <Trash2 className="h-4 w-4" />
                          </IconButton>
                        </div>
                        <FieldDropList
                          droppableId={folderDroppableId(group.folder.id)}
                          fields={group.fields}
                          emptyHint="Drag fields here"
                        />
                      </div>
                    )}
                  </Draggable>
                ))}
                {provided.placeholder}
              </div>
            )}
          </Droppable>

          <div className="mt-4">
            <SectionLabel className="mb-2">Unassigned</SectionLabel>
            <div className="rounded-lg border border-dashed border-gray-200 bg-gray-50">
              <FieldDropList
                droppableId={UNASSIGNED_DROPPABLE}
                fields={view.unassigned}
                emptyHint="No unassigned fields"
              />
            </div>
          </div>
        </DragDropContext>

        {!hasFields && view.folders.length === 0 && (
          <EmptyState title="No folders yet" subtitle="Create a folder, then drag fields into it." />
        )}
      </Card>

      <DeletePipelineResourceConfirmDialog
        open={folderToDelete != null}
        onClose={() => setFolderToDelete(null)}
        title="Delete folder?"
        subtitle={
          folderToDelete
            ? `Delete "${folderToDelete.name}"? Fields inside it will move to Unassigned.`
            : ""
        }
        loading={deleteFolder.isPending}
        onConfirm={() => {
          if (!folderToDelete) return;
          deleteFolder.mutate(folderToDelete.id, {
            onSuccess: () => {
              toast.success("Folder deleted");
              setFolderToDelete(null);
            },
            onError: (e) => toast.error(errorMessage(e)),
          });
        }}
      />
    </>
  );
}

function FieldDropList({
  droppableId,
  fields,
  emptyHint,
}: {
  droppableId: string;
  fields: CustomField[];
  emptyHint: string;
}) {
  return (
    <Droppable droppableId={droppableId} type="field">
      {(provided, snapshot) => (
        <div
          ref={provided.innerRef}
          {...provided.droppableProps}
          className={cn(
            "min-h-12 space-y-1.5 p-2",
            snapshot.isDraggingOver && "bg-jade-50"
          )}
        >
          {fields.length === 0 && !snapshot.isDraggingOver && (
            <p className="px-2 py-1.5 text-xs text-gray-400">{emptyHint}</p>
          )}
          {fields.map((f, index) => (
            <Draggable key={f.id} draggableId={`field-${f.id}`} index={index}>
              {(drag, dragSnapshot) => (
                <div
                  ref={drag.innerRef}
                  {...drag.draggableProps}
                  className={cn(
                    "flex items-center gap-2 rounded-md border border-gray-200 bg-white px-2 py-1.5 text-sm",
                    dragSnapshot.isDragging && "shadow-sm"
                  )}
                >
                  <span
                    {...drag.dragHandleProps}
                    className="cursor-grab text-gray-400 hover:text-gray-600"
                  >
                    <GripVertical className="h-4 w-4" />
                  </span>
                  <span className="min-w-0 flex-1 truncate text-gray-800">{f.name}</span>
                  <span className="shrink-0 font-mono text-xs text-gray-400">{f.field_key}</span>
                </div>
              )}
            </Draggable>
          ))}
          {provided.placeholder}
        </div>
      )}
    </Droppable>
  );
}
