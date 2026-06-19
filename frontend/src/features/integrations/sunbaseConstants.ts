import type { OutboundFieldMapEntry } from "@/types";

export const SUNBASE_URL =
  "https://server4.sunbasedata.com/sunbase/portal/api/lead_post.jsp";

export const SUNBASE_OUTBOUND_BUILTINS = [
  "first_name",
  "last_name",
  "phone",
  "email",
  "address",
  "city",
  "state",
  "zip",
  "action_at",
  "source",
  "external_id",
  "lead_id",
  "public_id",
  "status",
  "cost",
  "revenue",
];

export function sunbaseFieldMap(schemaName: string): OutboundFieldMapEntry[] {
  return [
    { dest_key: "schema_name", source_type: "static", static_value: schemaName },
    { dest_key: "last_name", source_type: "builtin", builtin_field: "last_name" },
    { dest_key: "first_name", source_type: "builtin", builtin_field: "first_name" },
    { dest_key: "address1", source_type: "builtin", builtin_field: "address" },
    { dest_key: "city", source_type: "builtin", builtin_field: "city" },
    { dest_key: "state", source_type: "builtin", builtin_field: "state" },
    { dest_key: "zip_code", source_type: "builtin", builtin_field: "zip" },
    { dest_key: "email", source_type: "builtin", builtin_field: "email" },
    { dest_key: "phone", source_type: "builtin", builtin_field: "phone" },
    { dest_key: "lead_source", source_type: "builtin", builtin_field: "source" },
    { dest_key: "lead_other", source_type: "builtin", builtin_field: "external_id" },
  ];
}

export function syncSchemaInFieldMap(
  entries: OutboundFieldMapEntry[],
  schemaName: string
): OutboundFieldMapEntry[] {
  return entries.map((e) =>
    e.dest_key === "schema_name" && e.source_type === "static"
      ? { ...e, static_value: schemaName }
      : e
  );
}
