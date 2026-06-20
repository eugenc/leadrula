import { useEffect, useMemo, useState } from "react";
import {
  DragDropContext,
  Droppable,
  Draggable,
  type DropResult,
  type DraggableProvidedDragHandleProps,
} from "@hello-pangea/dnd";
import { GripVertical, Lock, Trash2 } from "lucide-react";
import { useCustomFields, useCustomFieldFolders } from "@/features/leads/hooks";
import {
  useUpdateCustomFieldFolder,
  useDeleteCustomFieldFolder,
  useSaveCustomFieldLayout,
} from "@/features/admin/hooks";
import {
  applyContactSystemDrag,
  applyDrag,
  applyFolderDrag,
  buildLayoutPayload,
  CONTACT_SYSTEM_DROPPABLE,
  folderDroppableId,
  groupCustomFieldsByFolder,
  splitFolderGroups,
  UNASSIGNED_DROPPABLE,
  type GroupedFields,
} from "@/features/admin/customFieldLayout";
import { DeletePipelineResourceConfirmDialog } from "@/features/pipelines/DeletePipelineResourceConfirmDialog";
import { Card, Spinner, EmptyState } from "@/components/ui/misc";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { IconButton } from "@/components/layout/IconButton";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { CustomField, CustomFieldFolder } from "@/types";
import {
  contactBuiltinOrderFromFolder,
  isContactFolder,
  orderedLockedContactSystemFields,
  orderedReorderableContactSystemFields,
  type ContactFieldKey,
} from "@/features/leads/contactSection";

export function CustomFieldFoldersTab() {
  const { data: folders, isLoading: foldersLoading } = useCustomFieldFolders();
  const { data: fields, isLoading: fieldsLoading } = useCustomFields();
  const updateFolder = useUpdateCustomFieldFolder();
  const deleteFolder = useDeleteCustomFieldFolder();
  const saveLayout = useSaveCustomFieldLayout();

  const [folderToDelete, setFolderToDelete] = useState<{ id: number; name: string } | null>(null);

  const serverGrouped = useMemo(
    () => groupCustomFieldsByFolder(folders ?? [], (fields ?? []).filter((f) => f.is_active)),
    [folders, fields]
  );
  const serverContactOrder = useMemo(() => {
    const contact = serverGrouped.folders.find((g) => isContactFolder(g.folder));
    return contactBuiltinOrderFromFolder(contact?.folder);
  }, [serverGrouped]);

  const [draft, setDraft] = useState<GroupedFields | null>(null);
  const [contactOrderDraft, setContactOrderDraft] = useState<ContactFieldKey[] | null>(null);
  useEffect(() => {
    setDraft(null);
    setContactOrderDraft(null);
  }, [serverGrouped, serverContactOrder.join(",")]);

  const view = draft ?? serverGrouped;
  const contactOrder = contactOrderDraft ?? serverContactOrder;
  const { contact: contactGroup, others: otherFolderGroups } = splitFolderGroups(view);
  function save(nextGrouped: GroupedFields, nextContactOrder: ContactFieldKey[]) {
    saveLayout.mutate(buildLayoutPayload(nextGrouped, nextContactOrder), {
      onError: (e) => {
        toast.error(errorMessage(e));
        setDraft(null);
        setContactOrderDraft(null);
      },
    });
  }

  function onDragEnd(result: DropResult) {
    if (result.type === "contact-system") {
      const nextOrder = applyContactSystemDrag(contactOrder, result);
      if (!nextOrder) return;
      setContactOrderDraft(nextOrder);
      save(view, nextOrder);
      return;
    }
    if (result.type === "folder") {
      const next = applyFolderDrag(view, result);
      if (!next) return;
      setDraft(next);
      save(next, contactOrder);
      return;
    }
    const next = applyDrag(view, result);
    if (!next) return;
    setDraft(next);
    save(next, contactOrder);
  }

  if (foldersLoading || fieldsLoading) return <Spinner className="h-6 w-6" />;

  const hasFields = view.folders.some((g) => g.fields.length) || view.unassigned.length > 0;

  return (
    <>
      <Card className="p-4">
        <DragDropContext onDragEnd={onDragEnd}>
          {contactGroup && (
            <div className="mb-4">
              <FolderHeading folder={contactGroup.folder} />
              <ContactSystemFieldList order={contactOrder} />
              <FieldDropList
                droppableId={folderDroppableId(contactGroup.folder.id)}
                fields={contactGroup.fields}
                emptyHint="Drag fields here"
              />
            </div>
          )}

          <Droppable droppableId="folder-list" type="folder">
            {(provided) => (
              <div ref={provided.innerRef} {...provided.droppableProps} className="space-y-4">
                {otherFolderGroups.map((group, index) => (
                  <Draggable key={group.folder.id} draggableId={`folder-${group.folder.id}`} index={index}>
                    {(drag, snapshot) => (
                      <div
                        ref={drag.innerRef}
                        {...drag.draggableProps}
                        className={cn(snapshot.isDragging && "rounded-lg bg-surface-card shadow-md")}
                      >
                        <FolderHeading
                          folder={group.folder}
                          dragHandleProps={drag.dragHandleProps}
                          onRename={(name) =>
                            updateFolder.mutate(
                              { id: group.folder.id, body: { name } },
                              { onError: (err) => toast.error(errorMessage(err)) }
                            )
                          }
                          onDelete={() => setFolderToDelete({ id: group.folder.id, name: group.folder.name })}
                        />
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
            <div className="rounded-lg border border-dashed border-gray-200 bg-surface-app">
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

function FolderHeading({
  folder,
  dragHandleProps,
  onRename,
  onDelete,
}: {
  folder: CustomFieldFolder;
  dragHandleProps?: DraggableProvidedDragHandleProps | null;
  onRename?: (name: string) => void;
  onDelete?: () => void;
}) {
  const contact = isContactFolder(folder);
  const [editing, setEditing] = useState(false);
  const submit = (value: string) => {
    const name = value.trim();
    setEditing(false);
    if (name && name !== folder.name && onRename) onRename(name);
  };
  return (
    <div className="mb-2 flex items-center gap-1.5">
      {dragHandleProps ? (
        <span {...dragHandleProps} className="cursor-grab text-gray-400 hover:text-gray-600">
          <GripVertical className="h-4 w-4" />
        </span>
      ) : (
        <span className="h-4 w-4 shrink-0" aria-hidden />
      )}
      {contact ? (
        <span className="text-xs font-semibold uppercase tracking-wide text-gray-400">Contact</span>
      ) : editing ? (
        <Input
          autoFocus
          defaultValue={folder.name}
          className="h-7 max-w-xs text-sm"
          onBlur={(e) => submit(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") e.currentTarget.blur();
            else if (e.key === "Escape") setEditing(false);
          }}
        />
      ) : (
        <button
          type="button"
          onClick={() => setEditing(true)}
          className="text-xs font-semibold uppercase tracking-wide text-gray-400 hover:text-gray-600"
        >
          {folder.name}
        </button>
      )}
      {!contact && onDelete && (
        <IconButton variant="danger" className="ml-auto" onClick={onDelete}>
          <Trash2 className="h-4 w-4" />
        </IconButton>
      )}
    </div>
  );
}

function ContactSystemFieldList({ order }: { order: ContactFieldKey[] }) {
  const lockedFields = orderedLockedContactSystemFields();
  const reorderableFields = orderedReorderableContactSystemFields(order);
  return (
    <div className="min-h-12 space-y-1.5 p-2">
      {lockedFields.map((f) => (
        <div
          key={f.key}
          className="flex items-center gap-2 rounded-md border border-gray-200 bg-surface-card px-2 py-1.5 text-sm opacity-60"
        >
          <Lock className="h-4 w-4 shrink-0 text-gray-400" />
          <span className="text-gray-800">{f.label}</span>
          <span className="ml-auto font-mono text-xs text-gray-400">{f.key}</span>
        </div>
      ))}
      <Droppable droppableId={CONTACT_SYSTEM_DROPPABLE} type="contact-system">
        {(provided) => (
          <div ref={provided.innerRef} {...provided.droppableProps} className="space-y-1.5">
            {reorderableFields.map((f, index) => (
              <Draggable key={f.key} draggableId={`contact-system-${f.key}`} index={index}>
                {(drag, dragSnapshot) => (
                  <div
                    ref={drag.innerRef}
                    {...drag.draggableProps}
                    className={cn(
                      "flex items-center gap-2 rounded-md border border-gray-200 bg-surface-card px-2 py-1.5 text-sm",
                      dragSnapshot.isDragging && "shadow-sm"
                    )}
                  >
                    <span
                      {...drag.dragHandleProps}
                      className="cursor-grab text-gray-400 hover:text-gray-600"
                    >
                      <GripVertical className="h-4 w-4" />
                    </span>
                    <span className="text-gray-800">{f.label}</span>
                    <span className="ml-auto font-mono text-xs text-gray-400">{f.key}</span>
                  </div>
                )}
              </Draggable>
            ))}
            {provided.placeholder}
          </div>
        )}
      </Droppable>
    </div>
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
                    "flex items-center gap-2 rounded-md border border-gray-200 bg-surface-card px-2 py-1.5 text-sm",
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
