# Leadrula Calendars API

Reference for Leadrula calendar and appointment booking APIs.

**Base URL:** `{API_BASE}` — your deployment API host (e.g. `https://api.example.com`).

---

## Overview

Leadrula has three calendar-related systems:

| System | Auth | Base path | Purpose |
|--------|------|-----------|---------|
| **Public appointment API** | API key | `/api/v1/appointments/*`, `/api/v1/booking-calendars/*` | Integrations (VoiceUni, etc.) |
| **Session appointment API** | JWT | `/publisher/*`, `/buyer/*` | Web app and logged-in agents |
| **Lead follow-up calendar** | JWT (buyer only) | `/buyer/calendar/*` | Callbacks/tasks from `leads.action_at` |

---

## Response envelope

**Success:**

```json
{ "data": { ... } }
```

**Error:**

```json
{
  "error": {
    "code": "validation_error",
    "message": "contract_id required"
  }
}
```

| Code | HTTP | Meaning |
|------|------|---------|
| `validation_error` | 400 | Invalid input |
| `unauthorized` | 401 | Missing or invalid credentials |
| `forbidden` | 403 | Insufficient scope or permission |
| `not_found` | 404 | Resource not found |
| `conflict` | 409 | State conflict |
| `business_rule` | 422 | Business logic rejection |
| `internal` | 500 | Server error |

---

## Appointment concepts

### Calendar source

Each **Appointment** contract has one active booking calendar:

```
appointment_calendar_source = "buyer" | "publisher"
```

| Source | Owner | Setup |
|--------|-------|-------|
| `buyer` | Buyer's booking calendar | Buyer selects source + calendar in Leadrula |
| `publisher` | Publisher's booking calendar | Publisher attaches calendar to contract; buyer switches source |

**Slot toggles per contract:**
- **Buyer source:** publisher enables buyer slots via `PUT /publisher/contracts/{id}/appointment-slots`
- **Publisher source:** buyer enables publisher slots via `PUT /buyer/contracts/{id}/publisher-appointment-slots`

### Booking rules

| Rule | Value |
|------|-------|
| Advance window | Up to **90 days** |
| Past slots | Rejected |
| Slot duration | 15–180 minutes |
| Capacity | 1–20 per slot instance |
| `date` query param | `YYYY-MM-DD` in calendar timezone |
| `slot_start` | RFC3339; must match slot template exactly |
| Slot ID when booking | `buyer_slot_id` if source=`buyer`; `publisher_slot_id` if source=`publisher` |
| Rebooking | Replaces any existing appointment for the same lead |

### Weekly schedule format

Booking calendars store working hours in `schedule` JSON:

```json
{
  "mon": { "start": "09:00", "end": "17:00" },
  "tue": { "start": "09:00", "end": "17:00" }
}
```

Weekday keys: `sun`–`sat`. Slot `weekday`: **0 = Sunday, 6 = Saturday**.

---

## Public API (API key)

For machine-to-machine integrations. Generate keys in **Settings → API**.

### Authentication

```http
Authorization: Bearer {prefix}.{secret}
```

**Scopes:**

| Scope | Access |
|-------|--------|
| `appointments:read` | List contracts, slots, markers, calendars, bookings |
| `appointments:write` | Book appointments (includes read) |

Publisher keys use publisher logic; buyer keys use buyer logic.

### Endpoints

| Method | Path | Scope |
|--------|------|-------|
| GET | `/api/v1/appointments/contracts` | read |
| GET | `/api/v1/appointments/slots?contract_id=&date=` | read |
| GET | `/api/v1/appointments/calendar-markers?contract_id=&from=&to=` | read |
| POST | `/api/v1/appointments/book` | write |
| GET | `/api/v1/appointments/booked` | read |
| GET | `/api/v1/booking-calendars` | read |
| GET | `/api/v1/booking-calendars/{id}` | read |
| GET | `/api/v1/booking-calendars/{id}/slots` | read |

### Booking flow

```
1. GET  /api/v1/appointments/contracts
2. GET  /api/v1/appointments/slots?contract_id={id}&date=YYYY-MM-DD
3. POST /api/v1/appointments/book
```

**Book request:**

```json
{
  "contract_id": 123,
  "publisher_slot_id": 456,
  "slot_start": "2026-08-15T14:00:00-04:00",
  "delivery_mode": "contract",
  "lead_id": 789,
  "first_name": "Jane",
  "last_name": "Doe",
  "phone": "+15551234567",
  "email": "jane@example.com",
  "source": "integration",
  "publisher_pipeline_id": 0,
  "publisher_stage_id": 0
}
```

| Field | Required | Notes |
|-------|----------|-------|
| `contract_id` | Yes | Active Appointment contract |
| `buyer_slot_id` | When source=`buyer` | From slots response |
| `publisher_slot_id` | When source=`publisher` | From slots response |
| `slot_start` | Yes | Exact value from slots response |
| `delivery_mode` | Publisher only | `"contract"` or `"publisher_pipeline"` |
| `lead_id` | Lead or contact | Existing lead |
| `first_name`, `last_name` | When no `lead_id` | Required for new lead |
| `phone` or `email` | When no `lead_id` | At least one required |

**Example:**

```bash
curl -s "$API_BASE/api/v1/appointments/contracts" \
  -H "Authorization: Bearer YOUR_API_KEY"

curl -s "$API_BASE/api/v1/appointments/slots?contract_id=1&date=2026-08-15" \
  -H "Authorization: Bearer YOUR_API_KEY"

curl -s -X POST "$API_BASE/api/v1/appointments/book" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"contract_id":1,"publisher_slot_id":456,"slot_start":"2026-08-15T14:00:00-04:00","delivery_mode":"contract","first_name":"Jane","last_name":"Doe","phone":"+15551234567"}'
```

See also: [VoiceUni integration guide](integrations/voiceuni-calendars-api.md).

---

## Session API (JWT)

For logged-in users in the Leadrula web app or agents using session tokens.

### Authentication

```http
POST {API_BASE}/auth/login
Content-Type: application/json

{ "email": "agent@example.com", "password": "..." }
```

Use `Authorization: Bearer {access}` on all requests. Refresh via `POST /auth/refresh`.

| Requirement | Details |
|-------------|---------|
| Publisher routes | JWT for a **publisher** account |
| Buyer routes | JWT for a **buyer** account |
| Book appointment | Role `admin` or `user` |
| Calendar CRUD / contract config | Permission `appointments_manage` |

---

## Publisher endpoints

Base: `{API_BASE}/publisher`

### Appointment booking

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/appointments/contracts` | JWT | List bookable contracts |
| GET | `/appointments/slots?contract_id=&date=` | JWT | Free slots for a day |
| GET | `/appointments/calendar-markers?contract_id=&from=&to=` | JWT | Month view markers |
| POST | `/appointments/book` | JWT admin/user | Book appointment |
| GET | `/appointments/booked` | JWT | List publisher bookings |

### Booking calendar management

| Method | Path | Auth |
|--------|------|------|
| GET | `/booking-calendars` | JWT |
| POST | `/booking-calendars` | JWT appointments_manage |
| GET | `/booking-calendars/{calendarId}` | JWT |
| PUT | `/booking-calendars/{calendarId}` | JWT appointments_manage |
| GET | `/booking-calendars/{calendarId}/slots` | JWT |
| POST | `/booking-calendars/{calendarId}/slots` | JWT appointments_manage |
| PATCH | `/booking-calendars/{calendarId}/slots/{slotId}` | JWT appointments_manage |
| POST | `/booking-calendars/{calendarId}/slots/copy` | JWT appointments_manage |
| GET | `/booking-calendars/{calendarId}/markers?from=&to=` | JWT |

### Contract configuration

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| PATCH | `/contracts/{id}/appointment-calendar` | JWT appointments_manage | Attach publisher calendar |
| GET | `/contracts/{id}/appointment-slots` | JWT | List buyer slots for contract |
| PUT | `/contracts/{id}/appointment-slots` | JWT appointments_manage | Toggle buyer slots (when source=buyer) |

---

## Buyer endpoints

Base: `{API_BASE}/buyer`

### Appointment booking

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/appointments/contracts` | JWT | List bookable contracts |
| GET | `/appointments/slots?contract_id=&date=` | JWT | Free slots |
| GET | `/appointments/calendar-markers?contract_id=&from=&to=` | JWT | Month view markers |
| POST | `/appointments/book` | JWT admin/user | Book (delivery_mode forced to `contract`) |
| GET | `/appointments` | JWT | List bookings (paginated) |

**List appointments query params:** `page`, `limit`, `contract_id`, `publisher_id`, `q`, `sort`, `sort_dir`, `appointment_preset`

### Booking calendar management

Same CRUD as publisher under `/buyer/booking-calendars/*`, plus:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/booking-calendars/{calendarId}/appointments` | Bookings for a calendar |

### Contract configuration

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| PATCH | `/contracts/{id}/appointment-calendar` | JWT appointments_manage | Set buyer calendar |
| PATCH | `/contracts/{id}/appointment-calendar-source` | JWT appointments_manage | Pick `buyer` or `publisher` source |
| GET | `/contracts/{id}/publisher-appointment-slots` | JWT | List publisher slots |
| PUT | `/contracts/{id}/publisher-appointment-slots` | JWT appointments_manage | Toggle publisher slots |

**Set calendar source (buyer):**

```json
{ "source": "buyer", "appointment_calendar_id": 5 }
```

**Switch to publisher calendar:**

```json
{ "source": "publisher" }
```

---

## Lead follow-up calendar (buyer only)

Events from leads with a scheduled `action_at` (callbacks, tasks). JWT buyer session required.

| Method | Path | Query |
|--------|------|-------|
| GET | `/buyer/calendar/global` | `from`, `to` (RFC3339, optional) |
| GET | `/buyer/calendar/me` | Same; scoped to current user |

**Event object:**

```json
{
  "lead_id": 12345,
  "title": "Jane Doe",
  "stage_id": 10,
  "pipeline_id": 2,
  "user_id": 5,
  "action_at": "2026-08-15T14:00:00Z",
  "overdue": false
}
```

---

## Shared data types

### FreeSlot

```json
{
  "buyer_slot_id": 10,
  "publisher_slot_id": 0,
  "slot_start": "2026-08-15T14:00:00-04:00",
  "duration_min": 30,
  "capacity": 2,
  "remaining_capacity": 1
}
```

Only one slot ID field is populated, based on active calendar source.

### BookingCalendar

```json
{
  "id": 1,
  "account_id": 42,
  "name": "Main Calendar",
  "schedule": { "mon": { "start": "09:00", "end": "17:00" } },
  "timezone": "America/New_York",
  "buffer_min": 15,
  "location": "Virtual",
  "configured": true,
  "slot_count": 5,
  "updated_at": "2026-08-01T12:00:00Z"
}
```

### CalendarDayMarker

```json
{
  "date": "2026-08-15",
  "has_bookable": true,
  "has_bookings": false
}
```

### BookingRow

```json
{
  "id": 999,
  "contract_id": 123,
  "contract_name": "Acme Appointments",
  "lead_id": 456,
  "lead_name": "Jane Doe",
  "phone": "+15551234567",
  "email": "jane@example.com",
  "booked_at": "2026-08-07T16:00:00Z",
  "appointment_at": "2026-08-15T14:00:00-04:00",
  "duration_min": 30,
  "delivery_mode": "contract",
  "delivery_status": "pending",
  "buyer_name": "Acme Corp",
  "publisher_name": "LeadGen Inc",
  "lead_status": "distributed"
}
```

### AppointmentContract (publisher view)

```json
{
  "contract_id": 123,
  "contract_name": "Acme Appointments",
  "buyer_id": 42,
  "buyer_name": "Acme Corp",
  "timezone": "America/New_York",
  "location": "Virtual",
  "configured": true,
  "calendar_source": "publisher"
}
```

### BuyerAppointmentContract (buyer view)

```json
{
  "contract_id": 123,
  "contract_name": "Acme Appointments",
  "publisher_id": 7,
  "publisher_name": "LeadGen Inc",
  "timezone": "America/New_York",
  "configured": true,
  "calendar_source": "buyer"
}
```

---

## Common errors

| Message | Cause |
|---------|-------|
| `missing appointments:read scope` | API key lacks appointment read scope |
| `missing appointments:write scope` | API key lacks appointment write scope |
| `appointment calendar is not configured` | Contract missing calendar setup |
| `buyer has not selected an appointment calendar` | No `appointment_calendar_source` set |
| `slot is outside booking window` | Slot >90 days ahead or in the past |
| `slot is full` | Capacity exhausted |
| `slot_start does not match slot template` | Wrong time sent — use exact value from slots response |
| `buyer_slot_id is required` | Wrong slot ID field for buyer source |
| `publisher_slot_id is required` | Wrong slot ID field for publisher source |
| `account has no admin users` | Public book failed — account needs an admin user |

---

## Source files

- Public API: `backend/internal/appointments/public_api.go`
- Session routes: `backend/internal/appointments/handler.go`
- Booking logic: `backend/internal/appointments/book.go`
- Calendar resolution: `backend/internal/appointments/calendar_resolve.go`
- Lead calendar: `backend/internal/calendar/calendar.go`
- In-app API docs: `frontend/src/features/api-docs/endpoints.ts`
