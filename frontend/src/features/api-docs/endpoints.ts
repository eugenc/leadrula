export type HttpMethod = "GET" | "POST" | "PATCH" | "PUT" | "DELETE";

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
        { name: "updated_since", description: "RFC3339 timestamp — return leads updated at or after this time (incremental sync)" },
        { name: "external_id", description: "Filter or lookup by VoiceUni/external CRM id" },
        { name: "all", description: "Set to 1 to return all leads (no pagination cap behavior)" },
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
      method: "PATCH",
      path: "/api/v1/leads/{public_id}",
      auth: "API key (leads:write)",
      description:
        "Partially update a lead. Publisher keys only. Lookup by path public_id or query ?external_id= for VoiceUni ids.",
      request: `curl -s -X PATCH "${baseURL}/api/v1/leads/550e8400-e29b-41d4-a716-446655440000" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "first_name": "Jane",
    "tags": ["voiceuni"],
    "stage_id": 10,
    "custom": { "utility_provider": "Example Co" }
  }'`,
      response: `{ "data": { "public_id": "...", "first_name": "Jane", ... } }`,
    },
    {
      method: "GET",
      path: "/api/v1/custom-fields",
      auth: "API key (leads:read or leads:write)",
      description: "List active custom field definitions for field mapping in CRM integrations.",
      response: `{ "data": { "items": [{ "id": 1, "field_key": "utility_provider", "name": "Utility Provider", "type": "text" }] } }`,
      request: `curl -s "${baseURL}/api/v1/custom-fields" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
    },
    {
      method: "POST",
      path: "/api/v1/integrations/voiceuni/ingest",
      auth: "API key (leads:write)",
      description:
        "Upsert a lead from VoiceUni by external_id. Applies the VoiceUni integration field map and routing. Publisher keys only.",
      request: `curl -s -X POST "${baseURL}/api/v1/integrations/voiceuni/ingest" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "connection_id": "550e8400-e29b-41d4-a716-446655440000",
    "external_id": "vu-lead-uuid",
    "first_name": "Jane",
    "last_name": "Doe",
    "phone": "+15551234567",
    "source": "voiceuni"
  }'`,
      response: `{ "data": { "lead_id": "...", "public_id": "...", "status": "distributed", "created": true } }`,
    },
    {
      method: "GET",
      path: "/api/v1/appointments/contracts",
      auth: "API key (appointments:read or appointments:write)",
      description:
        "List bookable appointment contracts. Publisher keys return buyer-facing contract info; buyer keys return publisher-facing contract info. Requires configured calendar on each contract.",
      response: `{ "data": { "items": [{ "contract_id": 1, "contract_name": "...", "configured": true, "calendar_source": "buyer" }] } }`,
      request: `curl -s "${baseURL}/api/v1/appointments/contracts" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
    },
    {
      method: "GET",
      path: "/api/v1/appointments/slots",
      auth: "API key (appointments:read or appointments:write)",
      description: "List free appointment slots for a contract on a given date.",
      queryParams: [
        { name: "contract_id", description: "Appointment contract id (required)" },
        { name: "date", description: "Date in YYYY-MM-DD (required, calendar timezone)" },
      ],
      response: `{ "data": { "items": [{ "buyer_slot_id": 10, "slot_start": "2026-08-15T14:00:00-04:00", "duration_min": 30, "remaining_capacity": 1 }] } }`,
      request: `curl -s "${baseURL}/api/v1/appointments/slots?contract_id=1&date=2026-08-15" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
    },
    {
      method: "GET",
      path: "/api/v1/appointments/calendar-markers",
      auth: "API key (appointments:read or appointments:write)",
      description: "Day markers for a contract calendar (month view).",
      queryParams: [
        { name: "contract_id", description: "Appointment contract id (required)" },
        { name: "from", description: "Start date YYYY-MM-DD (required)" },
        { name: "to", description: "End date YYYY-MM-DD (required)" },
      ],
      response: `{ "data": { "items": [{ "date": "2026-08-15", "has_bookable": true, "has_bookings": false }] } }`,
      request: `curl -s "${baseURL}/api/v1/appointments/calendar-markers?contract_id=1&from=2026-08-01&to=2026-08-31" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
    },
    {
      method: "POST",
      path: "/api/v1/appointments/book",
      auth: "API key (appointments:write)",
      description:
        "Book an appointment slot. Use buyer_slot_id or publisher_slot_id from the slots response (based on contract calendar_source). Optional external_event_id for VoiceUni dedup.",
      request: `curl -s -X POST "${baseURL}/api/v1/appointments/book" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "contract_id": 1,
    "publisher_slot_id": 456,
    "slot_start": "2026-08-15T14:00:00-04:00",
    "delivery_mode": "contract",
    "first_name": "Jane",
    "last_name": "Doe",
    "phone": "+15551234567",
    "source": "voiceuni",
    "external_event_id": "vu-appt-uuid"
  }'`,
      response: `{ "data": { "id": 999, "contract_id": 1, "lead_id": 456, "appointment_at": "2026-08-15T14:00:00-04:00", "external_event_id": "vu-appt-uuid" } }`,
    },
    {
      method: "GET",
      path: "/api/v1/appointments/booked",
      auth: "API key (appointments:read or appointments:write)",
      description: "List booked appointments. Supports from/to date window and contract_id filter.",
      queryParams: [
        { name: "from", description: "Start of appointment window (YYYY-MM-DD or RFC3339)" },
        { name: "to", description: "End of appointment window (YYYY-MM-DD or RFC3339)" },
        { name: "contract_id", description: "Filter by contract" },
        { name: "limit", description: "Max rows (publisher default 100, max 500)" },
        { name: "page", description: "Page number (buyer, default 1)" },
        { name: "publisher_id", description: "Filter by publisher (buyer)" },
        { name: "appointment_preset", description: "Buyer preset: today, this_week, this_month, all (ignored when from/to set)" },
        { name: "q", description: "Search lead name/phone/email (buyer)" },
      ],
      response: `{ "data": { "items": [{ "id": 999, "lead_name": "Jane Doe", "appointment_at": "..." }] } }`,
      request: `curl -s "${baseURL}/api/v1/appointments/booked" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
    },
    {
      method: "GET",
      path: "/api/v1/booking-calendars",
      auth: "API key (appointments:read or appointments:write)",
      description: "List booking calendars for the API key account (read-only). Includes contract_id when exactly one contract uses the calendar.",
      response: `{ "data": { "items": [{ "id": 1, "name": "Main", "timezone": "America/New_York", "configured": true, "contract_id": 123, "calendar_source": "publisher" }] } }`,
      request: `curl -s "${baseURL}/api/v1/booking-calendars" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
    },
    {
      method: "GET",
      path: "/api/v1/booking-calendars/{id}",
      auth: "API key (appointments:read or appointments:write)",
      description: "Get a booking calendar by id.",
      response: `{ "data": { "id": 1, "name": "Main", "schedule": {}, "timezone": "America/New_York" } }`,
      request: `curl -s "${baseURL}/api/v1/booking-calendars/1" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
    },
    {
      method: "GET",
      path: "/api/v1/booking-calendars/{id}/slots",
      auth: "API key (appointments:read or appointments:write)",
      description: "List recurring slot templates on a booking calendar.",
      response: `{ "data": { "items": [{ "id": 10, "weekday": 1, "start_time": "09:00:00", "duration_min": 30 }] } }`,
      request: `curl -s "${baseURL}/api/v1/booking-calendars/1/slots" \\
  -H "Authorization: Bearer YOUR_API_KEY"`,
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
      },
      {
        id: "booking-calendars",
        title: "Booking calendars",
        publisherOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/booking-calendars"), auth: "JWT", description: "List publisher booking calendars." },
          { method: "POST", path: nsPath(ns, "/booking-calendars"), auth: "JWT appointments_manage", description: "Create booking calendar." },
          { method: "PATCH", path: nsPath(ns, "/contracts/{id}/appointment-calendar"), auth: "JWT appointments_manage", description: "Attach publisher calendar to contract." },
          { method: "GET", path: nsPath(ns, "/appointments/contracts"), auth: "JWT", description: "List bookable appointment contracts." },
          { method: "POST", path: nsPath(ns, "/appointments/book"), auth: "JWT", description: "Book an appointment slot." },
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
        id: "booking-calendars",
        title: "Booking calendars",
        buyerOnly: true,
        endpoints: [
          { method: "GET", path: nsPath(ns, "/booking-calendars"), auth: "JWT", description: "List buyer booking calendars." },
          { method: "PATCH", path: nsPath(ns, "/contracts/{id}/appointment-calendar-source"), auth: "JWT appointments_manage", description: "Choose buyer or publisher calendar for contract." },
          { method: "GET", path: nsPath(ns, "/contracts/{id}/publisher-appointment-slots"), auth: "JWT", description: "Publisher calendar slots for contract." },
          { method: "PUT", path: nsPath(ns, "/contracts/{id}/publisher-appointment-slots"), auth: "JWT appointments_manage", description: "Toggle publisher slots on contract." },
          { method: "POST", path: nsPath(ns, "/appointments/book"), auth: "JWT", description: "Book an appointment as buyer." },
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
