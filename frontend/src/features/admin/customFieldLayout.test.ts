import { describe, expect, it } from "vitest";
import type { DropResult } from "@hello-pangea/dnd";
import type { CustomField, CustomFieldFolder } from "@/types";
import {
  applyDrag,
  buildLayoutPayload,
  groupCustomFieldsByFolder,
  UNASSIGNED_DROPPABLE,
  folderDroppableId,
} from "./customFieldLayout";

function field(id: number, overrides: Partial<CustomField> = {}): CustomField {
  return {
    id,
    name: `Field ${id}`,
    field_key: `field_${id}`,
    type: "text",
    options: [],
    position: id,
    is_active: true,
    folder_id: null,
    ...overrides,
  };
}

function folder(id: number, position: number): CustomFieldFolder {
  return { id, name: `Folder ${id}`, position };
}

describe("groupCustomFieldsByFolder", () => {
  it("buckets fields under folders in order and leaves the rest unassigned", () => {
    const folders = [folder(2, 1), folder(1, 0)];
    const fields = [
      field(10, { folder_id: 1, position: 1 }),
      field(11, { folder_id: 1, position: 0 }),
      field(20, { folder_id: 2, position: 0 }),
      field(30, { folder_id: null, position: 5 }),
    ];

    const grouped = groupCustomFieldsByFolder(folders, fields);

    expect(grouped.folders.map((g) => g.folder.id)).toEqual([1, 2]);
    expect(grouped.folders[0]!.fields.map((f) => f.id)).toEqual([11, 10]);
    expect(grouped.folders[1]!.fields.map((f) => f.id)).toEqual([20]);
    expect(grouped.unassigned.map((f) => f.id)).toEqual([30]);
  });

  it("treats a field pointing at a missing folder as unassigned", () => {
    const grouped = groupCustomFieldsByFolder([folder(1, 0)], [field(10, { folder_id: 99 })]);
    expect(grouped.folders[0]!.fields).toHaveLength(0);
    expect(grouped.unassigned.map((f) => f.id)).toEqual([10]);
  });
});

describe("applyDrag", () => {
  const grouped = groupCustomFieldsByFolder(
    [folder(1, 0), folder(2, 1)],
    [field(10, { folder_id: 1, position: 0 }), field(20, { folder_id: null, position: 1 })]
  );

  it("moves a field from unassigned into a folder", () => {
    const result = {
      type: "field",
      source: { droppableId: UNASSIGNED_DROPPABLE, index: 0 },
      destination: { droppableId: folderDroppableId(2), index: 0 },
    } as DropResult;

    const next = applyDrag(grouped, result);
    expect(next).not.toBeNull();
    expect(next!.unassigned).toHaveLength(0);
    expect(next!.folders[1]!.fields.map((f) => f.id)).toEqual([20]);
  });

  it("reorders folders", () => {
    const result = {
      type: "folder",
      source: { droppableId: "folder-list", index: 0 },
      destination: { droppableId: "folder-list", index: 1 },
    } as DropResult;

    const next = applyDrag(grouped, result);
    expect(next!.folders.map((g) => g.folder.id)).toEqual([2, 1]);
  });

  it("returns null for a no-op drag", () => {
    const result = {
      type: "field",
      source: { droppableId: folderDroppableId(1), index: 0 },
      destination: { droppableId: folderDroppableId(1), index: 0 },
    } as DropResult;
    expect(applyDrag(grouped, result)).toBeNull();
  });
});

describe("buildLayoutPayload", () => {
  it("emits folder order and globally increasing field positions", () => {
    const grouped = groupCustomFieldsByFolder(
      [folder(1, 0), folder(2, 1)],
      [
        field(10, { folder_id: 1, position: 0 }),
        field(11, { folder_id: 1, position: 1 }),
        field(20, { folder_id: 2, position: 0 }),
        field(30, { folder_id: null, position: 0 }),
      ]
    );

    const payload = buildLayoutPayload(grouped);

    expect(payload.folders).toEqual([
      { id: 1, position: 0 },
      { id: 2, position: 1 },
    ]);
    expect(payload.fields).toEqual([
      { id: 10, folder_id: 1, position: 0 },
      { id: 11, folder_id: 1, position: 1 },
      { id: 20, folder_id: 2, position: 2 },
      { id: 30, folder_id: null, position: 3 },
    ]);
  });
});
