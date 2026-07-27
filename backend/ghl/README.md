# GHL setup scripts (Nuve / NC)

Local-only helpers for GoHighLevel API setup. **Do not commit real tokens** — `pit_nc` is gitignored.

## PIT scopes required

| Task | Scopes |
|------|--------|
| Pipelines | `opportunities.write`, `opportunities.readonly` |
| Calendars | `calendars.write`, `calendars.readonly` |
| Custom fields (field mapping dropdown) | `locations/customFields.read` |

In GHL: **Settings → Private Integrations → [your token] → Scopes**. Add missing scopes and save. If GHL regenerates the token, update `pit_nc`.

## Nuve location

- **Location ID:** `EK5QVQpSm7djvzTkQNIP`
- **Pipeline:** Nuve Leads (`jjFgv4ewhw0aO9ziDEUO`)

## Create calendar (Mon–Sat 9am–8pm)

```bash
chmod +x backend/ghl/create_nuve_calendar.sh
./backend/ghl/create_nuve_calendar.sh
```

Optional env vars:

- `TIMEZONE` — default `America/New_York`
- `CALENDAR_NAME` — default `Nuve Appointments`

After success, copy the printed `calendar_id` into the Leadrula GHL connection config.
