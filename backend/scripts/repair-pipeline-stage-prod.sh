#!/usr/bin/env bash
# Diagnose and repair pipeline/stage mismatches on Railway production Postgres.
# Requires: railway login, run from backend/ (linked to leadrula production).
set -euo pipefail
cd "$(dirname "$0")/.."

if ! railway whoami &>/dev/null; then
  echo "Run: railway login" >&2
  exit 1
fi

export DATABASE_URL
DATABASE_URL="$(railway variables --json | python3 -c "import sys,json; print(json.load(sys.stdin)['DATABASE_PUBLIC_URL'])")"

if [[ $# -eq 0 ]]; then
  set -- -first Eugene -last Testter -repair
fi

go run ./cmd/repair-pipeline-stage "$@"
