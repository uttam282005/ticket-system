# Ticket System — Implementation Spec (Golang)

Build exactly what's described here. Do not add features, endpoints, or fields beyond this spec — the grading is an automated hidden test suite that checks exact paths, field names, and status codes.

---

## 1. Stack

- **Language**: Go (use modules, `go.mod`)
- **Router**: `chi` or `gorilla/mux` or stdlib `net/http` with `ServeMux` — pick one, keep it simple
- **DB**: PostgreSQL (use `pgx` or `database/sql` + `lib/pq`). Do NOT use in-memory or SQLite even though the brief allows it — PostgreSQL is required for this build since Docker Compose will run both.
- **Auth**: JWT via `github.com/golang-jwt/jwt/v5`
- **Password hashing**: `golang.org/x/crypto/bcrypt`
- **Migrations**: plain SQL files run on startup, or a lightweight tool like `golang-migrate`. Keep it simple — a single `schema.sql` executed on boot is fine.
- **Config**: environment variables only, no config files. Provide `.env.example`.

## 2. Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── auth/          # JWT generation/validation, password hashing
│   ├── handlers/       # HTTP handlers (auth, tickets, health)
│   ├── middleware/     # JWT auth middleware
│   ├── models/         # User, Ticket structs
│   ├── store/           # DB access layer (queries)
│   └── db/               # connection setup, migrations
├── migrations/
│   └── 001_init.sql
├── Dockerfile
├── docker-compose.yml   # app + postgres, for local dev only
├── .env.example
├── go.mod
├── go.sum
└── README.md
```

## 3. Data Model

### users
| column | type | notes |
|---|---|---|
| id | UUID (or serial int) | primary key |
| email | text | unique, not null |
| password_hash | text | bcrypt hash, never returned in any response |
| created_at | timestamptz | default now() |

### tickets
| column | type | notes |
|---|---|---|
| id | UUID (or serial int) | primary key |
| user_id | FK -> users.id | owner, not null |
| title | text | not null |
| description | text | nullable |
| status | text | enum: `open`, `in_progress`, `closed`. default `open` |
| created_at | timestamptz | default now() |
| updated_at | timestamptz | updated on every write |

Use whichever ID type (UUID vs int) you want, but be consistent — `{id}` in ticket routes must match whatever type you return from creation.

## 4. Auth Rules

- Register: email + password. Reject duplicate emails with `409 Conflict`.
- Passwords: bcrypt hash before storing, cost factor 10–12. Never log or return raw passwords.
- Login: verify bcrypt hash, issue JWT on success.
- JWT claims: at minimum `sub` (user id) and `exp`. Sign with `HS256` using a secret from env var `JWT_SECRET`. Expiry: 24h (put this in a const, doesn't matter exactly).
- Protected routes require header `Authorization: Bearer <token>`. Middleware must:
  - Reject missing header → `401`
  - Reject malformed header (no "Bearer " prefix) → `401`
  - Reject invalid/expired token → `401`
  - On success, inject user ID into request context for handlers to use.

## 5. Endpoints — Exact Contract

All responses are `application/json`. All request bodies are JSON.

### `GET /health`
No auth required.
Response `200`:
```json
{ "status": "ok" }
```

### `POST /auth/register`
Request:
```json
{ "email": "user@example.com", "password": "plaintext" }
```
- `201` on success:
```json
{ "id": "...", "email": "user@example.com" }
```
- `400` if email or password missing/invalid format
- `409` if email already registered

### `POST /auth/login`
Request:
```json
{ "email": "user@example.com", "password": "plaintext" }
```
- `200` on success:
```json
{ "token": "<jwt>" }
```
- `401` on wrong email/password (don't leak which one is wrong)

### `POST /tickets` (protected)
Request:
```json
{ "title": "Something broke", "description": "optional details" }
```
- `201` on success, returns the created ticket:
```json
{
  "id": "...",
  "user_id": "...",
  "title": "Something broke",
  "description": "optional details",
  "status": "open",
  "created_at": "...",
  "updated_at": "..."
}
```
- `400` if title missing/empty

### `GET /tickets` (protected)
Returns only tickets owned by the authenticated user.
- `200`:
```json
{ "tickets": [ { ...ticket }, { ...ticket } ] }
```
(empty array, not null, if user has none)

### `GET /tickets/{id}` (protected)
- `200` with the ticket if it exists AND belongs to the caller
- `404` if it doesn't exist, OR if it belongs to a different user (do NOT return 403 here — return 404 so a user can't probe for the existence of other users' ticket IDs)

### `PATCH /tickets/{id}/status` (protected)
Request:
```json
{ "status": "in_progress" }
```
- `200` with updated ticket on success
- `404` if ticket doesn't exist or isn't owned by caller (same reasoning as above)
- `400` if `status` value isn't one of `open`, `in_progress`, `closed`
- `409` if the transition is invalid:
  - Valid forward transitions only: `open → in_progress`, `in_progress → closed`
  - `closed` is terminal — any update attempt on a closed ticket returns `409`
  - Skipping a step (`open → closed` directly) — decide and document this in the README (spec doesn't explicitly forbid it, but the safest interpretation given "Required Status Flow" is sequential only: reject `open → closed` directly with `409`). Implement it as sequential-only.

## 6. Ownership Enforcement

Every ticket read/write handler must filter by `user_id = <authenticated user>` at the query level (in the SQL `WHERE` clause), not just in application logic after fetching. This is both a correctness and security requirement.

## 7. Error Response Shape

Use a consistent JSON error shape across all endpoints:
```json
{ "error": "human readable message" }
```
Use correct status codes throughout (400/401/404/409/500). Don't leak stack traces or raw DB errors to the client — log them server-side, return a generic `500` message.

## 8. Dockerfile

- Multi-stage build: build stage compiles the Go binary, final stage is a minimal image (`alpine` or `scratch` + certs).
- Expose port `8080`.
- Must build and run standalone with:
```
docker build -t ticket-system .
docker run -p 8080:8080 ticket-system
```
- Since the app needs Postgres, also provide a `docker-compose.yml` for local dev (app + postgres), but the standalone `docker run` command above must still work — meaning the app needs a way to get its DB connection string from env vars (`DATABASE_URL`), and README should note that a reachable Postgres instance (via `DATABASE_URL`) is required even for the bare `docker run` case.

## 9. Deployment

- Deploy to any free-tier host that supports Docker/Go (e.g. Render, Railway, Fly.io — pick one with a genuinely free tier as of today, verify before committing).
- Use a free managed Postgres instance (Render/Railway/Neon/Supabase all have free tiers) and set `DATABASE_URL` as an env var on the host.
- `/health` must be publicly reachable with no auth.
- Confirm the deployed base URL exposes the exact same routes as local.

## 10. README Requirements

Must include:
- Local run: `go run ./cmd/server` and Docker run commands
- `docker build` / `docker run` commands (copy from section 8)
- Environment variables required (`JWT_SECRET`, `DATABASE_URL`, `PORT` if used) with a link to `.env.example`
- Deployed URL and deployed `/health` URL
- Any assumptions made (e.g. the sequential-only status transition decision from section 5, ID type chosen, JWT expiry duration)

## 11. .env.example

```
DATABASE_URL=postgres://user:password@localhost:5432/tickets?sslmode=disable
JWT_SECRET=change-me
PORT=8080
```

## 12. Out of Scope (do not build)

- No admin role
- No ticket assignment
- No comments
- No pagination, filtering, or search on `/tickets` (unless trivially easy — but not required, skip it)
- No password reset / email verification

## 13. Definition of Done

- [ ] All 7 endpoints implemented exactly as specified (paths, methods, status codes, JSON shapes)
- [ ] JWT auth middleware correctly protects all non-health, non-auth routes
- [ ] Passwords bcrypt-hashed, never returned in any response
- [ ] Ownership checks enforced at the DB query level
- [ ] Status transition state machine enforced server-side with correct 409s
- [ ] `go vet` / `go build` clean, no unused imports, reasonable package structure per section 2
- [ ] Dockerfile builds and runs per section 8's exact commands
- [ ] Service deployed, `/health` publicly reachable
- [ ] README complete per section 10
- [ ] `.env.example` present
