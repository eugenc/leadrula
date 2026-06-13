export type HttpMethod = "GET" | "POST" | "PATCH" | "DELETE";

export type DocEndpoint = {
  method: HttpMethod;
  path: string;
  auth: string;
  description: string;
  request?: string;
  response?: string;
  queryParams?: { name: string; description: string }[];
};

export type DocGroup = {
  id: string;
  title: string;
  description?: string;
  publisherOnly?: boolean;
  buyerOnly?: boolean;
  endpoints: DocEndpoint[];
};

export const OUTBOUND_TRIGGER_EVENTS = [
  { id: "lead.create", label: "Lead created" },
  { id: "lead.update", label: "Lead updated" },
  { id: "lead.delete", label: "Lead deleted" },
  { id: "pipeline.move_stage", label: "Lead moved to another stage" },
  { id: "pipeline.place", label: "Lead placed on a pipeline" },
  { id: "pipeline.stage_rule_applied", label: "Stage rule applied" },
] as const;

export const ERROR_CODES = [
  "unauthorized",
  "forbidden",
  "not_found",
  "validation_error",
  "conflict",
  "business_rule",
  "insufficient_balance",
  "internal",
  "service_unavailable",
] as const;

export function publicEndpoints(baseURL: string): DocEndpoint[] {
  return [
    {
      method: "GET",
      path: "/api/v1/leads",
      auth: "API key (leads:read or leads:write)",
      description: "List and search leads owned by the API key account.",
      queryParams: [
        { name: "q", description: "Free-text search across name, phone, email, address, public_id, status, and more" },
        { name: "phone", description: "Exact phone match (returns one lead or 404)" },
        { name: "email", description: "Exact email match (returns one lead or 404)" },
        { name: "status", description: "Filter by lead status" },
        { name: "source", description: "Filter by source slug (alias: campaign)" },
        { name: "pipeline_id", description: "Filter by pipeline" },
        { name: "stage_id", description: "Filter by stage" },
        { name: "assigned", description: "Filter by assigned user id" },
        { name: "tag", description: "Filter by tag" },
        { name: "page", description: "Page number (default 1)" },
        { name: "limit", description: "Page size" },
        { name: "sort", description: "Sort column (e.g. created_at, first_name, phone)" },
        { name: "sort_dir", description: "asc or desc" },
      ],
      response: `{
  "data": {
    "items": [{ "public_id": "...", "first_name": "Jane", ... }],
    "total": 42,
    "page": 1,
    "limit": 50
  }
}`,
      request: `curl -s "${baseURL}/api/v1/leads?q=Jane" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
    },
    {
      method: "GET",
      path: "/api/v1/leads/{public_id}",
      auth: "API key (leads:read or leads:write)",
      description: "Get a single lead by public UUID.",
      response: `{
  "data": {
    "public_id": "550e8400-e29b-41d4-a716-446655440000",
    "first_name": "Jane",
    "last_name": "Doe",
    "phone": "+15551234567",
    "status": "distributed",
    ...
  }
}`,
      request: `curl -s "${baseURL}/api/v1/leads/550e8400-e29b-41d4-a716-446655440000" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
    },
    {
      method: "POST",
      path: "/api/v1/leads",
      auth: "API key (leads:write)",
      description: "Ingest a lead into the intake queue. Publisher accounts only.",
      request: `curl -s -X POST "${baseURL}/api/v1/leads" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "source": "web_form",
    "first_name": "Jane",
    "last_name": "Doe",
    "phone": "+15551234567",
    "email": "jane@example.com",
    "custom": { "utility_provider": "Example Co" }
  }'`,
      response: `{
  "data": {
    "lead_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "distributed"
  }
}`,
    },
    {
      method: "POST",
      path: "/api/v1/leads/{public_id}/action",
      auth: "API key (leads:write)",
      description: "Set or clear the follow-up action date on a lead.",
      request: `curl -s -X POST "${baseURL}/api/v1/leads/550e8400-e29b-41d4-a716-446655440000/action" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{ "action_at": "2026-06-15T14:00:00Z" }'`,
      response: `{ "data": { "ok": true } }`,
    },
    {
      method: "POST",
      path: "/api/v1/sources/{slug}",
      auth: "API key if source requires it",
      description: "Ingest a lead via a configured source slug. Publisher accounts only.",
      request: `curl -s -X POST "${baseURL}/api/v1/sources/my-source" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{ "first_name": "Jane", "phone": "+15551234567" }'`,
      response: `{ "data": { "lead_id": "...", "status": "review" } }`,
    },
    {
      method: "POST",
      path: "/api/v1/webhooks/{slug}",
      auth: "Webhook secret if required",
      description: "Send an inbound webhook payload. Field mapping rules are configured in the Webhooks page.",
      request: `curl -s -X POST "${baseURL}/api/v1/webhooks/my-webhook" \\
  -H "Authorization: Bearer WEBHOOK_SECRET" \\
  -H "Content-Type: application/json" \\
  -d '{ "first_name": "Jane", "phone": "+15551234567" }'`,
      response: `{ "data": { "lead_id": "...", "status": "distributed" } }`,
    },
  ];
}

function nsPath(ns: string, path: string) {
  return `${ns}${path}`;
}

export function jwtEndpointGroups(ns: "/publisher" | "/buyer"): DocGroup[] {
  const isPublisher = ns === "/publisher";
  const groups: DocGroup[] = [
    {
      id: "auth",
      title: "Auth & profile",
      description: "Session endpoints (no namespace prefix).",
      endpoints: [
        { method: "POST", path: "/auth/login", auth: "None", description: "Email/password login. Returns access and refresh tokens." },
        { method: "POST", path: "/auth/refresh", auth: "None", description: "Exchange refresh token for a new access token." },
        { method: "POST", path: "/auth/logout", auth: "None", description: "Client-side logout (discard tokens)." },
        { method: "GET", path: "/auth/me", auth: "JWT", description: "Current user and account profile." },
        { method: "PATCH", path: "/auth/me/prefs", auth: "JWT", description: "Update user preferences." },
        { method: "PATCH", path: "/auth/me/account", auth: "JWT admin", description: "Update account settings." },
        { method: "GET", path: "/auth/switchable", auth: "JWT", description: "List accounts the user can switch to." },
        { method: "POST", path: "/auth/switch", auth: "JWT", description: "Switch active account context." },
        { method: "POST", path: "/auth/switch-back", auth: "JWT", description: "Return to previous account." },
      ],
    },
    {
      id: "leads",
      title: "Leads",
      description: "Lead CRUD, search, views, notes, and followers.",
      endpoints: [
        {
          method: "GET",
          path: nsPath(ns, "/leads"),
          auth: "JWT",
          description: "List and filter leads. Supports free-text search via q (name, phone, email, public_id, etc.).",
          queryParams: [
            { name: "q", description: "Free-text search" },
            { name: "status", description: "Lead status filter" },
            { name: "source", description: "Source slug" },
            { name: "pipeline_id", description: "Pipeline id" },
            { name: "stage_id", description: "Stage id" },
            { name: "page", description: "Page number" },
            { name: "limit", description: "Page size" },
            { name: "view_id", description: "Apply a saved view's filters" },
          ],
        },
        {
          method: "GET",
          path: nsPath(ns, "/leads/{id}"),
          auth: "JWT",
          description: "Get lead detail. {id} may be numeric id or public_id UUID.",
        },
        { method: "POST", path: nsPath(ns, "/leads"), auth: "JWT admin/user", description: "Create a lead manually." },
        { method: "POST", path: nsPath(ns, "/leads/import"), auth: "JWT admin/user", description: "Bulk import leads." },
        { method: "PATCH", path: nsPath(ns, "/leads/{id}"), auth: "JWT", description: "Update lead fields, assignee, tags, custom values." },
        { method: "PATCH", path: nsPath(ns, "/leads/{id}/stage"), auth: "JWT", description: "Move lead to another stage." },
        { method: "PATCH", path: nsPath(ns, "/leads/{id}/action"), auth: "JWT", description: "Set follow-up action date." },
        { method: "DELETE", path: nsPath(ns, "/leads/{id}"), auth: "JWT admin", description: "Soft-delete a lead." },
        { method: "GET", path: nsPath(ns, "/leads/{id}/notes"), auth: "JWT", description: "List notes on a lead." },
        { method: "POST", path: nsPath(ns, "/leads/{id}/notes"), auth: "JWT", description: "Add a note." },
        { method: "GET", path: nsPath(ns, "/leads/views"), auth: "JWT", description: "List saved lead views." },
        { method: "POST", path: nsPath(ns, "/leads/views"), auth: "JWT", description: "Create a saved view." },
        ...(isPublisher
          ? [{ method: "POST" as const, path: nsPath(ns, "/leads/{id}/redistribute"), auth: "JWT admin", description: "Redistribute a lead to another buyer." }]
          : []),
      ],
    },
    {
      id: "pipelines",
      title: "Pipelines & stages",
      endpoints: [
        { method: "GET", path: nsPath(ns, "/pipelines"), auth: "JWT", description: "List pipelines." },
        { method: "POST", path: nsPath(ns, "/pipelines"), auth: "JWT admin", description: "Create pipeline." },
        { method: "PATCH", path: nsPath(ns, "/pipelines/{id}"), auth: "JWT admin", description: "Update pipeline." },
        { method: "DELETE", path: nsPath(ns, "/pipelines/{id}"), auth: "JWT admin", description: "Delete pipeline." },
        { method: "GET", path: nsPath(ns, "/pipelines/{id}/stages"), auth: "JWT", description: "List stages." },
        { method: "POST", path: nsPath(ns, "/pipelines/{id}/stages"), auth: "JWT admin", description: "Create stage." },
        { method: "PATCH", path: nsPath(ns, "/stages/{id}"), auth: "JWT admin", description: "Update stage." },
        { method: "GET", path: nsPath(ns, "/stages/{id}/rules"), auth: "JWT", description: "List stage automation rules." },
      ],
    },
    {
      id: "custom-fields",
      title: "Custom fields",
      endpoints: [
        { method: "GET", path: nsPath(ns, "/custom-fields"), auth: "JWT", description: "List custom fields." },
        { method: "POST", path: nsPath(ns, "/custom-fields"), auth: "JWT admin", description: "Create custom field." },
        { method: "PATCH", path: nsPath(ns, "/custom-fields/{id}"), auth: "JWT admin", description: "Update custom field." },
        { method: "DELETE", path: nsPath(ns, "/custom-fields/{id}"), auth: "JWT admin", description: "Delete custom field." },
      ],
    },
    {
      id: "webhooks",
      title: "Webhooks",
      endpoints: [
        { method: "GET", path: nsPath(ns, "/webhooks"), auth: "JWT", description: "List webhooks." },
        { method: "POST", path: nsPath(ns, "/webhooks"), auth: "JWT admin", description: "Create webhook." },
        { method: "PATCH", path: nsPath(ns, "/webhooks/{id}"), auth: "JWT admin", description: "Update webhook." },
        { method: "DELETE", path: nsPath(ns, "/webhooks/{id}"), auth: "JWT admin", description: "Delete webhook." },
        { method: "GET", path: nsPath(ns, "/webhooks/{id}/events"), auth: "JWT", description: "List inbound event handlers." },
        { method: "GET", path: nsPath(ns, "/webhooks/{id}/outbound-triggers"), auth: "JWT", description: "List outbound triggers." },
        { method: "POST", path: nsPath(ns, "/webhooks/{id}/outbound-triggers"), auth: "JWT admin", description: "Create outbound trigger." },
        { method: "GET", path: nsPath(ns, "/webhook-deliveries"), auth: "JWT", description: "List all delivery log entries." },
        { method: "GET", path: nsPath(ns, "/webhooks/{id}/deliveries"), auth: "JWT", description: "Delivery log for one webhook." },
        { method: "POST", path: nsPath(ns, "/webhooks/{id}/deliveries/{deliveryId}/replay"), auth: "JWT admin", description: "Replay a failed delivery." },
      ],
    },
    {
      id: "api-keys",
      title: "API keys",
      endpoints: [
        { method: "GET", path: nsPath(ns, "/api-keys"), auth: "JWT", description: "List API keys." },
        { method: "POST", path: nsPath(ns, "/api-keys"), auth: "JWT admin", description: "Create API key. Returns secret once." },
        { method: "PATCH", path: nsPath(ns, "/api-keys/{id}"), auth: "JWT admin", description: "Rename API key." },
        { method: "POST", path: nsPath(ns, "/api-keys/{id}/rotate"), auth: "JWT admin", description: "Rotate API key." },
        { method: "DELETE", path: nsPath(ns, "/api-keys/{id}"), auth: "JWT admin", description: "Revoke API key." },
      ],
    },
    {
      id: "users",
      title: "Users",
      endpoints: [
        { method: "GET", path: nsPath(ns, "/users"), auth: "JWT", description: "List users." },
        { method: "POST", path: nsPath(ns, "/users/invite"), auth: "JWT admin", description: "Invite user." },
        { method: "PATCH", path: nsPath(ns, "/users/{id}"), auth: "JWT admin", description: "Update user." },
        { method: "DELETE", path: nsPath(ns, "/users/{id}"), auth: "JWT admin", description: "Delete user." },
      ],
    },
    {
      id: "notifications",
      title: "Notifications",
      endpoints: [
        { method: "GET", path: nsPath(ns, "/notifications"), auth: "JWT", description: "List notifications." },
        { method: "GET", path: nsPath(ns, "/notifications/settings"), auth: "JWT", description: "Get notification preferences." },
        { method: "PATCH", path: nsPath(ns, "/notifications/settings"), auth: "JWT", description: "Update notification preferences." },
      ],
    },
    {
      id: "dashboard",
      title: "Dashboard views",
      endpoints: [
        { method: "GET", path: nsPath(ns, "/dashboard/views"), auth: "JWT", description: "List dashboard views." },
        { method: "POST", path: nsPath(ns, "/dashboard/views"), auth: "JWT admin", description: "Create dashboard view." },
        { method: "PATCH", path: nsPath(ns, "/dashboard/views/{viewId}"), auth: "JWT admin", description: "Update dashboard view." },
      ],
    },
  ];

  if (isPublisher) {
    groups.push(
      {
        id: "sources",
        title: "Sources & routing",
        publisherOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/sources"), auth: "JWT", description: "List intake sources." },
          { method: "POST", path: nsPath(ns, "/sources"), auth: "JWT admin", description: "Create source." },
          { method: "GET", path: nsPath(ns, "/routes"), auth: "JWT", description: "List routing rules." },
          { method: "POST", path: nsPath(ns, "/routes"), auth: "JWT admin", description: "Create routing rule." },
          { method: "GET", path: nsPath(ns, "/intake-queue"), auth: "JWT", description: "Pending intake queue." },
          { method: "GET", path: nsPath(ns, "/inbound-log"), auth: "JWT", description: "Inbound request log." },
        ],
      },
      {
        id: "contracts",
        title: "Contracts",
        publisherOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/contracts"), auth: "JWT", description: "List contracts." },
          { method: "POST", path: nsPath(ns, "/contracts"), auth: "JWT admin", description: "Create contract." },
          { method: "GET", path: nsPath(ns, "/contracts/{id}"), auth: "JWT", description: "Get contract." },
          { method: "GET", path: nsPath(ns, "/payouts/summary"), auth: "JWT", description: "Payout summary." },
          { method: "GET", path: nsPath(ns, "/buyers"), auth: "JWT", description: "List managed buyers." },
        ],
      },
      {
        id: "billing",
        title: "Billing",
        publisherOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/billing/transactions"), auth: "JWT", description: "Buyer transaction ledger." },
          { method: "GET", path: nsPath(ns, "/billing/invoices"), auth: "JWT admin", description: "List invoices." },
          { method: "POST", path: nsPath(ns, "/billing/invoices"), auth: "JWT admin", description: "Create invoice." },
          { method: "GET", path: nsPath(ns, "/billing/stripe/status"), auth: "JWT admin", description: "Stripe Connect status." },
        ],
      },
      {
        id: "integrations",
        title: "Integrations",
        publisherOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/integrations/providers"), auth: "JWT", description: "List integration providers." },
          { method: "GET", path: nsPath(ns, "/integrations/connections"), auth: "JWT", description: "List connections." },
          { method: "POST", path: nsPath(ns, "/integrations/connections"), auth: "JWT admin", description: "Create connection." },
        ],
      }
    );
  } else {
    groups.push(
      {
        id: "contracts",
        title: "Contracts & participations",
        buyerOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/contracts"), auth: "JWT", description: "List contracts." },
          { method: "GET", path: nsPath(ns, "/participations"), auth: "JWT", description: "List participations." },
          { method: "POST", path: nsPath(ns, "/participations/{id}/accept"), auth: "JWT admin", description: "Accept participation." },
        ],
      },
      {
        id: "billing",
        title: "Billing",
        buyerOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/billing/balance"), auth: "JWT", description: "Account balance." },
          { method: "GET", path: nsPath(ns, "/billing/transactions"), auth: "JWT", description: "Transaction ledger." },
          { method: "GET", path: nsPath(ns, "/billing/invoices"), auth: "JWT", description: "List invoices." },
          { method: "POST", path: nsPath(ns, "/billing/balance/topup-intent"), auth: "JWT", description: "Create balance top-up." },
        ],
      },
      {
        id: "calendar",
        title: "Calendar",
        buyerOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/calendar/global"), auth: "JWT", description: "All account calendar events." },
          { method: "GET", path: nsPath(ns, "/calendar/me"), auth: "JWT", description: "Current user's calendar events." },
        ],
      },
      {
        id: "routes",
        title: "Routes",
        buyerOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/routes"), auth: "JWT", description: "List inbound routes (read-only)." },
          { method: "GET", path: nsPath(ns, "/routing-log"), auth: "JWT admin", description: "Contract routing log." },
          { method: "GET", path: nsPath(ns, "/inbound-log"), auth: "JWT admin", description: "Webhooks and CRM integration delivery log." },
        ],
      },
      {
        id: "integrations",
        title: "Integrations",
        buyerOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/integrations/providers"), auth: "JWT", description: "List integration providers." },
          { method: "GET", path: nsPath(ns, "/integrations/connections"), auth: "JWT", description: "List connections." },
        ],
      }
    );
  }

  return groups;
}
