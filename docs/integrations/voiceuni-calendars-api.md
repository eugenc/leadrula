# LeadRula ↔ VoiceUni Integration API

Public `/api/v1` endpoints for VoiceUni dialer integration using a single publisher API key.

## Authentication

Generate an API key in LeadRula (**Settings → API**) with scopes:

| Scope | Use |
|-------|-----|
| `leads:read` | List/get leads, custom fields |
| `leads:write` | Ingest, patch leads, VoiceUni ingest |
| `appointments:read` | Calendars, slots, booked list |
| `appointments:write` | Book appointments (includes read) |

```http
Authorization: Bearer {prefix}.{secret}
```

## Setup in LeadRula

1. **Integrations → VoiceUni** — create a connection (field map + routing via integration-origin routes).
2. Copy the **ingest endpoint** and `connection_id` from the connection settings.
3. Attach a route with origin **Integration → VoiceUni connection** to auto-distribute leads.

## Contacts

### Push (VoiceUni → LeadRula)

```http
POST /api/v1/integrations/voiceuni/ingest
```

Upserts by `external_id` (VoiceUni lead UUID). Returns `public_id` for storage in VoiceUni.

### Update

```http
PATCH /api/v1/leads/{public_id}
PATCH /api/v1/leads/{public_id}?external_id={voiceuni_uuid}
```

Flat body: `first_name`, `last_name`, `phone`, `email`, `tags`, `stage_id`, `custom: { field_key: value }`.

### Pull / sync

```http
GET /api/v1/leads?updated_since=2026-08-01T00:00:00Z
GET /api/v1/leads?external_id={voiceuni_uuid}
GET /api/v1/custom-fields
```

Use max `updated_at` from each page as the next sync cursor.

## Calendars & appointments

See booking flow in the API docs UI. Summary:

1. `GET /api/v1/booking-calendars` — pick calendar (`contract_id` included when unambiguous)
2. `GET /api/v1/appointments/slots?contract_id=&date=`
3. `POST /api/v1/appointments/book` — optional `external_event_id` for dedup
4. `GET /api/v1/appointments/booked?from=&to=&contract_id=` — sync window

Publisher calendar bookings use `publisher_slot_id`; buyer calendars use `buyer_slot_id`.

## Call transfer preload

```http
POST /api/v1/calls/preload
```

Requires a **call-type** routing source slug (Twilio-backed). The VoiceUni connection stores a suggested `call_source_slug` in config — link or create a call source in LeadRula routing, then pass that slug:

```json
{
  "source": "your-call-source-slug",
  "caller_phone": "+15551234567",
  "payload": {
    "disposition": "interested",
    "campaign": "Summer",
    "recording_url": "https://..."
  }
}
```

Returns `{ "preload_token", "expires_at" }` for the tracking URL.

## Error format

```json
{ "error": { "code": "validation_error", "message": "external_id is required" } }
```
