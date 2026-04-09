# Design: owner-web-panel

## Technical Approach

React (Vite) frontend served separately from the Go API. The Go API adds owner authentication endpoints and a CORS middleware. The React app communicates with the existing REST API using Bearer tokens stored in `localStorage`. A new `internal/owner/` package handles owner auth on the Go side. No server-rendered HTML.

**Ports:**
- Go API: `:8080` (existing)
- React dev server: `:5173` (Vite)
- Production: Go serves React's `dist/` as embedded static files on a separate port (e.g. `:3000`) or via a CDN

---

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|----------|--------|----------|-----------|
| Frontend | React + Vite + TypeScript | HTMX + html/template | Better UX for forms/tables/modals; user preference |
| Language | TypeScript | JavaScript | Type safety consistent with Go's strict typing |
| UI library | Tailwind CSS + shadcn/ui | Plain CSS, MUI | Ready-made components (Table, Dialog, Button); no custom design needed |
| Token storage | `localStorage` + Bearer header | httpOnly cookie | Simpler cross-origin auth; cookie SameSite issues with separate origins |
| CSRF | Not needed | Double-submit / signed field | Bearer tokens are not sent automatically by the browser — no CSRF risk |
| Frontend location | `web/` dir in same repo | Separate repo | Monorepo keeps API + UI changes in one PR; simpler CI |
| Prod static serving | Go embeds `web/dist/` on `:3000` | CDN / nginx | Single Docker image; no extra infra for MVP |
| CORS | Allowed origin from `CORS_ORIGIN` env var | Wildcard `*` | Wildcard blocks credentials; explicit origin required |

---

## Data Flow

**Login:**
```
React POST /auth/owner/login {email, password}
  → httpapi/handlers/owner_auth.go OwnerLogin()
  → owner.Service.Authenticate(email, password)  
      → bcrypt.CompareHashAndPassword()
  → auth.JWTService.Issue(Claims{role=admin, ownerID}, 8h)
  → 200 {token: "<jwt>"}
React stores token in localStorage
All subsequent requests: Authorization: Bearer <token>
```

**Approve expense:**
```
React PATCH /expenses/{id}/approve  Authorization: Bearer <token>
  → existing auth.RequireAuth(RoleAdmin) middleware (unchanged)
  → expense.Service.Approve(id, ownerID)
  → 200 {}
```

---

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `migrations/000008_add_owner_password_hash.up.sql` | Create | `ALTER TABLE owners ADD COLUMN password_hash TEXT NOT NULL DEFAULT ''` |
| `migrations/000008_add_owner_password_hash.down.sql` | Create | `ALTER TABLE owners DROP COLUMN password_hash` |
| `internal/owner/owner.go` | Create | Owner entity, Repository interface, Service interface, sentinel errors |
| `internal/owner/repository.go` | Create | pgx implementation |
| `internal/owner/service.go` | Create | Authenticate (bcrypt), Create (hash + store) |
| `internal/owner/service_test.go` | Create | Unit tests with mock repo |
| `internal/owner/mock_repository.go` | Create | testify mock |
| `internal/owner/repository_integration_test.go` | Create | testcontainers integration tests |
| `internal/httpapi/handlers/owner_auth.go` | Create | POST `/auth/owner/login`, POST `/auth/owner/bootstrap` |
| `internal/httpapi/handlers/owner_auth_test.go` | Create | Unit tests for owner auth handlers |
| `internal/httpapi/middleware/cors.go` | Create | CORS middleware: allowed origin from `CORS_ORIGIN` env, supports credentials |
| `internal/app/wire.go` | Modify | Add `OwnerRepo owner.Repository`, `OwnerSvc owner.Service` to `Deps` and `Build()` |
| `internal/config/config.go` | Modify | Add `BootstrapSecret string`, `CORSOrigin string` (optional) |
| `internal/httpapi/router.go` | Modify | Register `/auth/owner/login`, `/auth/owner/bootstrap`; add CORS middleware |
| `web/` | Create | React + Vite + TypeScript app (Tailwind CSS, shadcn/ui, React Router) |
| `web/src/pages/` | Create | Login, Dashboard, Taxis, Drivers, Expenses, Reports |
| `web/src/components/ui/` | Create | shadcn/ui components (Button, Table, Dialog, Badge, etc.) |
| `web/src/api/` | Create | Typed API client with Bearer token injection |
| `cmd/web/main.go` | Create | Static file server that embeds `web/dist/` on `:3000` |
| `docker-compose.yml` | Modify | Add `web` service on port 3000; add `CORS_ORIGIN` env to `api` service |
| `Dockerfile` | Modify | Add Node build stage for React; copy `web/dist/` into final image |

---

## Interfaces / Contracts

```go
// internal/owner/owner.go

type Owner struct {
    ID           uuid.UUID
    Name         string
    Email        string
    PasswordHash string
    CreatedAt    time.Time
}

var (
    ErrNotFound           = errors.New("owner not found")
    ErrInvalidCredentials = errors.New("invalid email or password")
    ErrDuplicateEmail     = errors.New("email already registered")
)

type Repository interface {
    Create(ctx context.Context, name, email, passwordHash string) (*Owner, error)
    GetByEmail(ctx context.Context, email string) (*Owner, error)
    Count(ctx context.Context) (int, error)
}

type Service interface {
    Create(ctx context.Context, name, email, plainPassword string) (*Owner, error)
    Authenticate(ctx context.Context, email, plainPassword string) (*Owner, error)
}
```

**New API endpoints:**
```
POST /auth/owner/login
  Body:    { "email": string, "password": string }
  200:     { "token": string }
  401:     invalid credentials

POST /auth/owner/bootstrap
  Header:  X-Bootstrap-Secret: <secret>
  Body:    { "name": string, "email": string, "password": string }
  201:     { "token": string }
  403:     wrong secret
  409:     owner already exists
```

---

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `owner.Service` Authenticate (correct/wrong/not-found) | testify mock repo |
| Unit | owner auth handlers (login, bootstrap) | `httptest.NewRecorder` |
| Integration | `owner.Repository` Create + GetByEmail | testcontainers postgres |
| Frontend | React components (login form, expense list) | Vitest + React Testing Library |

---

## Migration / Rollout

Migration 000008 adds `password_hash TEXT NOT NULL DEFAULT ''` to `owners`. Safe — no existing owner rows in production. Bootstrap endpoint creates the first owner with a real hash; the empty default is never used in practice.

## Open Questions

- [x] React UI library: Tailwind + shadcn/ui
- [x] TypeScript
- [ ] Should `cmd/web/main.go` serve on a configurable port via `WEB_PORT` env var?
