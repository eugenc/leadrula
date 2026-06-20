import type { DropResult } from "@hello-pangea/dnd";
import type { CustomField, CustomFieldFolder } from "@/types";
import type { CustomFieldLayoutPayload } from "./hooks";

export interface FolderGroup {
  folder: CustomFieldFolder;
  fields: CustomField[];
}

export interface GroupedFields {
  folders: FolderGroup[];
  unassigned: CustomField[];
}

export const UNASSIGNED_DROPPABLE = "unassigned";
export const folderDroppableId = (id: number) => `folder-${id}`;

/** Groups fields under their folder (in folder order), with the rest at the bottom. */
export function groupCustomFieldsByFolder(
  folders: CustomFieldFolder[],
  fields: CustomField[]
): GroupedFields {
  const sortedFolders = [...folders].sort((a, b) => a.position - b.position);
  const sortedFields = [...fields].sort((a, b) => a.position - b.position);

  const byFolder = new Map<number, CustomField[]>();
  sortedFolders.forEach((f) => byFolder.set(f.id, []));

  const unassigned: CustomField[] = [];
  for (const field of sortedFields) {
    const bucket = field.folder_id != null ? byFolder.get(field.folder_id) : undefined;
    if (bucket) bucket.push(field);
    else unassigned.push(field);
  }

  return {
    folders: sortedFolders.map((folder) => ({ folder, fields: byFolder.get(folder.id)! })),
    unassigned,
  };
}

/** Returns the new grouped state after a drag, or null if nothing changed. */
export function applyDrag(grouped: GroupedFields, result: DropResult): GroupedFields | null {
  const { source, destination, type } = result;
  if (!destination) return null;

  if (type === "folder") {
    if (destination.index === source.index) return null;
    const folders = [...grouped.folders];
    const [moved] = folders.splice(source.index, 1);
    folders.splice(destination.index, 0, moved!);
    return { ...grouped, folders };
  }

  if (destination.droppableId === source.droppableId && destination.index === source.index) {
    return null;
  }

  const next: GroupedFields = {
    folders: grouped.folders.map((g) => ({ folder: g.folder, fields: [...g.fields] })),
    unassigned: [...grouped.unassigned],
  };

  const listFor = (droppableId: string): CustomField[] => {
    if (droppableId === UNASSIGNED_DROPPABLE) return next.unassigned;
    const id = Number(droppableId.slice("folder-".length));
    return next.folders.find((g) => g.folder.id === id)!.fields;
  };

  const [moved] = listFor(source.droppableId).splice(source.index, 1);
  if (!moved) return null;
  listFor(destination.droppableId).splice(destination.index, 0, moved);
  return next;
}

/** Flattens grouped state into the layout payload with globally increasing field positions. */
export function buildLayoutPayload(grouped: GroupedFields): CustomFieldLayoutPayload {
  const folders = grouped.folders.map((g, i) => ({ id: g.folder.id, position: i }));
  const fields: CustomFieldLayoutPayload["fields"] = [];
  let position = 0;
  for (const g of grouped.folders) {
    for (const f of g.fields) fields.push({ id: f.id, folder_id: g.folder.id, position: position++ });
  }
  for (const f of grouped.unassigned) fields.push({ id: f.id, folder_id: null, position: position++ });
  return { folders, fields };
}
