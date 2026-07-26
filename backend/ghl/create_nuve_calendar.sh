#!/usr/bin/env bash
# Create Nuve GHL calendar with Mon-Sat 9am-8pm availability.
# Requires PIT with calendars.write scope (see backend/ghl/README.md).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOKEN="$(tr -d '[:space:]' < "$SCRIPT_DIR/pit_nc")"
LOCATION_ID="${LOCATION_ID:-EK5QVQpSm7djvzTkQNIP}"
TIMEZONE="${TIMEZONE:-America/New_York}"
BASE_URL="https://services.leadconnectorhq.com"
CALENDAR_NAME="${CALENDAR_NAME:-Nuve Appointments}"

if [[ -z "$TOKEN" ]]; then
  echo "Missing token in $SCRIPT_DIR/pit_nc" >&2
  exit 1
fi

ghl() {
  local method="$1"
  local path="$2"
  local version="$3"
  local body="${4:-}"
  local args=(
    -sS -w "\nHTTP:%{http_code}"
    -X "$method"
    -H "Authorization: Bearer $TOKEN"
    -H "Version: $version"
    -H "Accept: application/json"
  )
  if [[ -n "$body" ]]; then
    args+=(-H "Content-Type: application/json" -d "$body")
  fi
  /usr/bin/curl "${args[@]}" "$BASE_URL$path"
}

echo "Creating calendar: $CALENDAR_NAME"
create_resp="$(ghl POST "/calendars/" "v3" "$(/usr/bin/python3 - <<PY
import json
print(json.dumps({
    "isActive": True,
    "locationId": "$LOCATION_ID",
    "name": "$CALENDAR_NAME",
    "slug": "nuve-appointments",
    "widgetSlug": "nuve-appointments",
    "calendarType": "event",
    "widgetType": "classic",
    "eventTitle": "{{contact.name}}",
    "eventColor": "#039be5",
    "slotDuration": 30,
    "slotDurationUnit": "mins",
    "slotInterval": 30,
    "slotIntervalUnit": "mins",
    "slotBuffer": 0,
    "slotBufferUnit": "mins",
    "appoinmentPerSlot": 1,
    "autoConfirm": True,
    "allowReschedule": True,
    "allowCancellation": True,
    "locationConfigurations": [{"kind": "custom", "location": "Phone / In-home"}],
}))
PY
)")"

echo "$create_resp"
http_code="$(echo "$create_resp" | /usr/bin/tail -1 | /usr/bin/sed 's/HTTP://')"
body="$(echo "$create_resp" | /usr/bin/sed '$d')"

calendar_id="$(echo "$body" | /usr/bin/python3 -c "
import json, sys
data = json.load(sys.stdin)
cal = data.get('calendar') or data
print(cal.get('id', ''))
" 2>/dev/null || true)"

if [[ "$http_code" != "200" && "$http_code" != "201" ]]; then
  if echo "$body" | /usr/bin/grep -q "already exists"; then
    echo "Calendar may already exist; listing calendars..."
    list_resp="$(ghl GET "/calendars/?locationId=$LOCATION_ID" "2021-07-28")"
    echo "$list_resp"
    calendar_id="$(echo "$list_resp" | /usr/bin/sed '$d' | /usr/bin/python3 -c "
import json, sys
data = json.load(sys.stdin)
for cal in data.get('calendars', []):
    if cal.get('name') == '$CALENDAR_NAME':
        print(cal['id'])
        break
")"
  fi
fi

if [[ -z "$calendar_id" ]]; then
  echo "Failed to get calendar id" >&2
  exit 1
fi

echo "Calendar ID: $calendar_id"

schedule_body="$(/usr/bin/python3 - <<PY
import json
days = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday"]
rules = [{
    "type": "wday",
    "day": day,
    "intervals": [{"from": "09:00", "to": "20:00"}],
} for day in days]
print(json.dumps({"timezone": "$TIMEZONE", "rules": rules}))
PY
)"

echo "Setting availability Mon-Sat 9am-8pm ($TIMEZONE)..."
schedule_resp="$(ghl POST "/calendars/schedules/event-calendar/$calendar_id" "v3" "$schedule_body")"
echo "$schedule_resp"

schedule_http="$(echo "$schedule_resp" | /usr/bin/tail -1 | /usr/bin/sed 's/HTTP://')"
if [[ "$schedule_http" != "200" && "$schedule_http" != "201" ]]; then
  echo "Schedule creation failed (HTTP $schedule_http). Calendar was created; set hours manually in GHL." >&2
  exit 1
fi

echo "Done."
echo "calendar_id=$calendar_id"
echo "Use this ID in the Leadrula GHL connection calendar_id field."
