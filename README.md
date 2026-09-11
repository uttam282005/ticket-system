# Ticket Management System

A production-ready RESTful ticket management service implemented in Go and PostgreSQL, featuring JWT authentication, query-level ownership enforcement, and a robust status state machine.

---

## 1. Features & Architecture

- **Language & Framework**: Go with [`chi`](https://github.com/go-chi/chi/v5) router.
- **Database**: PostgreSQL with connection pooling via [`pgx/v5`](https://github.com/jackc/pgx/v5).
- **Authentication**: Stateless JWT via [`golang-jwt/jwt/v5`](https://github.com/golang-jwt/jwt/v5) (HMAC-SHA256).
- **Security**: Passwords hashed with [`bcrypt`](https://pkg.go.dev/golang.org/x/crypto/bcrypt) (cost factor 10). Password hashes are never exposed in any API response or logs.
- **Ownership Isolation**: Every ticket read and write query enforces `WHERE user_id = $2` at the database SQL query level. Requests for non-existent tickets or tickets owned by other users return `404 Not Found` (never `403`), preventing ID enumeration.
- **Status Lifecycle**: Once a ticket is `closed`, no further status changes are allowed (attempts return `409 Conflict`). Direct transitions (such as `open` → `closed`) and non-closed updates are supported.

---

## 2. API Endpoints

All request and response bodies use `application/json`.

| Method | Path | Auth Required | Description | Success Status |
|---|---|---|---|---|
| `GET` | `/health` | No | Service liveness healthcheck | `200 OK` |
| `POST` | `/auth/register` | No | Register a new user | `201 Created` |
| `POST` | `/auth/login` | No | Authenticate and obtain JWT token | `200 OK` |
| `POST` | `/tickets` | Yes (`Bearer <token>`) | Create a new ticket | `201 Created` |
| `GET` | `/tickets` | Yes (`Bearer <token>`) | List tickets owned by authenticated user | `200 OK` |
| `GET` | `/tickets/{id}` | Yes (`Bearer <token>`) | Fetch single ticket by ID (caller must own) | `200 OK` |
| `PATCH` | `/tickets/{id}/status` | Yes (`Bearer <token>`) | Update ticket status (`open`, `in_progress`, `closed`) | `200 OK` |

### Error Response Format
All errors return a consistent JSON payload:
```json
{
  "error": "human readable error message"
}
```

---

## 3. Environment Variables

Configuration is provided via environment variables only. See [.env.example](.env.example).

| Variable | Required | Description | Example |
|---|---|---|---|
| `DATABASE_URL` | Yes | PostgreSQL connection string | `postgres://user:password@localhost:5432/tickets?sslmode=disable` |
| `JWT_SECRET` | Yes | Secret key used to sign and verify JWT tokens | `super-secret-key-change-in-production` |

> **Note on Port**: Per specification, the service strictly binds to port `8080`. No `PORT` environment variable is read or supported.

---

## 4. Running Locally

### Option A: Using Docker Compose (App + PostgreSQL)

```bash
docker compose up --build
```
This boots a PostgreSQL database with health checks and runs the application listening on port `8080`.

### Option B: Running with Go directly

1. Ensure PostgreSQL is running and accessible via `DATABASE_URL`.
2. Export required environment variables:
   ```bash
   export DATABASE_URL="postgres://postgres:postgres@localhost:5432/tickets?sslmode=disable"
   export JWT_SECRET="my-local-secret"
   ```
3. Start the server:
   ```bash
   go run ./cmd/server
   ```
Migrations run automatically on boot.

---

## 5. Docker Build & Run (Standalone)

Per specification section 8:

```bash
docker build -t ticket-system .
docker run -p 8080:8080 -e DATABASE_URL="postgres://user:password@host.docker.internal:5432/tickets?sslmode=disable" -e JWT_SECRET="your-secret" ticket-system
```

*(Note: Ensure `DATABASE_URL` points to a reachable PostgreSQL instance when running standalone.)*

---

## 6. Testing & Verification

Run the test suite:
```bash
go test -v ./...
```

Run static analysis:
```bash
go vet ./...
```

---

## 7. Assumptions & Design Decisions

- **Listen Port**: The HTTP server listen port is strictly hard-coded to `:8080` per the assignment requirements and cannot be altered via environment variables.
- **Status State Machine**: Valid statuses are `open`, `in_progress`, and `closed`. Any transition is allowed except modifying an already-closed ticket, which returns `409 Conflict`. Direct transitions (such as `open` → `closed`) return `200 OK`. Any status outside the three valid values returns `400 Bad Request`.
- **Identifier Format**: UUIDv4 (`gen_random_uuid()`) is used consistently across users and tickets.
- **JWT Expiration**: Tokens expire 24 hours from issuance and carry standard claims (`sub` with the user ID, `exp`, and `iat`).
- **Data Isolation**: Query-level filtering (`WHERE user_id = $2`) guarantees callers never access tickets belonging to another user. If a ticket is not found or is owned by another user, `404 Not Found` is returned.

---

## 8. Deployment

- **Base URL**: https://ticket-booking-system-g1xe.onrender.com/
- **Health Check URL**: https://ticket-booking-system-g1xe.onrender.com/health
