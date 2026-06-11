# Lead Distribution CRM (leadrula)

A single-instance lead distribution CRM: one Publisher routes inbound leads to many Buyers,
who work the leads as cards on a Pipedrive-style Kanban board. Leads can be returned to the
publisher and re-distributed; buyers are charged a flat rate per distributed lead.

- **Backend:** Go 1.25, go-chi, pgx v5, JWT auth, argon2id passwords
- **Database:** PostgreSQL 15+ (local dev works on 14+)
- **Frontend:** React 19 + Vite, Tailwind (jade design tokens), TanStack Query + Zustand
- **Deploy:** Railway (Postgres + API via Dockerfile), Vercel (frontend SPA)

## Repository layout

```
leadrula/
├── backend/          # Go API + migrations + CLI commands
├── frontend/         # React SPA
├── docker-compose.yml  # local Postgres
└── .env.example
```

## Local development

### 1. Start Postgres

```bash
docker compose up -d
```

### 2. Backend

```bash
cd backend
cp ../.env.example .env        # adjust values
go run ./cmd/server            # runs migrations on boot, listens on :8080
```

Create the first publisher admin (reads BOOTSTRAP_* from env, or pass flags):

```bash
go run ./cmd/bootstrap
# or: go run ./cmd/bootstrap -email admin@example.com -password secret -name "Admin"
```

Optional demo data (buyers, contracts, pipelines, sample leads):

```bash
go run ./cmd/seed-demo
```

### 3. Frontend

```bash
cd frontend
npm install
echo "VITE_API_URL=http://localhost:8080" > .env
npm run dev                    # http://localhost:5173
```

## Deployment

### Railway (Postgres + Go API)

1. Add the **PostgreSQL** plugin — it injects `DATABASE_URL`.
2. Create a service with root directory `backend/`. It deploys via `backend/Dockerfile`
   (configured in `backend/railway.json`), which also builds the `bootstrap` and `seed-demo` binaries.
3. Set env vars: `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `APP_BASE_URL` (the Vercel URL),
   `API_BASE_URL` (the Railway API URL — same host as frontend `VITE_API_URL`; used for SunBase inbound webhook URLs shown in the UI),
   `CORS_ORIGINS` (the Vercel URL), and Mailgun API vars (`MAILGUN_API_KEY`, `MAILGUN_DOMAIN`, `MAILGUN_FROM`).
4. Migrations run automatically on startup (`/healthz` is the health check). Create the first
   publisher admin once by opening a shell on the service and running
   `BOOTSTRAP_EMAIL=... BOOTSTRAP_PASSWORD=... /app/bootstrap` (optionally `/app/seed-demo`).

### Vercel (frontend)

- Root directory: `frontend/`
- Build command: `npm run build`, output `dist/`
- Env: `VITE_API_URL=https://<your-railway-api-host>`

## Inbound lead API

```
POST /api/v1/leads
Authorization: Bearer <api_key>
{
  "source": "solar_ontario_q2",
  "first_name": "Jane", "last_name": "Doe",
  "phone": "+1...", "email": "...",
  "custom": { "utility_provider": "Hydro One" }
}
→ 202 { "lead_id": "<uuid>", "status": "distributed" | "review" }
```

`campaign_name` is still accepted for backward compatibility.

See [backend/README routes](backend) and the spec docs for the full surface.
