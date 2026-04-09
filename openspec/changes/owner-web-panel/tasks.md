# Tasks: owner-web-panel

## Phase 1: Foundation

- [x] 1.1 Create `migrations/000008_add_owner_password_hash.up.sql` — `ALTER TABLE owners ADD COLUMN password_hash TEXT NOT NULL DEFAULT ''`
- [x] 1.2 Create `migrations/000008_add_owner_password_hash.down.sql` — `ALTER TABLE owners DROP COLUMN password_hash`
- [x] 1.3 Create `internal/owner/owner.go` — Owner entity, Repository interface (Create, GetByEmail, Count), Service interface (Create, Authenticate, Count), sentinel errors (ErrNotFound, ErrInvalidCredentials, ErrDuplicateEmail)
- [x] 1.4 Add `BootstrapSecret string` and `CORSOrigin string` to `internal/config/config.go`; both optional (no validation required)

## Phase 2: Owner Domain (TDD)

- [x] 2.1 Create `internal/owner/mock_repository.go` — testify mock for OwnerRepository
- [x] 2.2 Create `internal/owner/service_test.go` — tests for Authenticate (correct password, wrong password, not found) and Create (success, duplicate email) — RED
- [x] 2.3 Create `internal/owner/service.go` — Authenticate via bcrypt, Create with bcrypt hash — GREEN
- [x] 2.4 Create `internal/owner/repository.go` — pgx implementation of OwnerRepository (Create, GetByEmail, Count)
- [ ] 2.5 Create `internal/owner/repository_integration_test.go` — testcontainers: Create + GetByEmail round-trip, duplicate email returns ErrDuplicateEmail

## Phase 3: API Layer (TDD)

- [x] 3.1 Create `internal/httpapi/middleware/cors.go` — CORS middleware reading `CORSOrigin` from config; allow credentials; handle preflight OPTIONS
- [x] 3.2 Create `internal/httpapi/handlers/owner_auth_test.go` — tests for POST `/auth/owner/login` (valid, invalid creds) and POST `/auth/owner/bootstrap` (valid secret, wrong secret, already exists) — RED
- [x] 3.3 Create `internal/httpapi/handlers/owner_auth.go` — OwnerLogin handler (200 `{token}` / 401) and OwnerBootstrap handler (201 / 403 / 409) — GREEN
- [x] 3.4 Add `OwnerRepo owner.Repository`, `OwnerSvc owner.Service` to `internal/app/wire.go` `Deps` struct and `Build()` function
- [x] 3.5 Modify `internal/httpapi/router.go` — add CORS middleware, register `POST /auth/owner/login` and `POST /auth/owner/bootstrap` as public routes; add `owner.Service` to `Services` struct

## Phase 4: React Scaffold

- [x] 4.1 Scaffold `web/` — `npm create vite@latest web -- --template react-ts`; install Tailwind CSS v4, shadcn/ui, React Router v6, Axios (note: downgraded to Vite 6 for Node 20.16 compatibility)
- [x] 4.2 Configure shadcn/ui (`npx shadcn@latest init`); add components: Button, Table, Dialog, Badge, Input, Label, Card
- [x] 4.3 Create `web/src/api/client.ts` — Axios instance with `Authorization: Bearer` interceptor reading token from localStorage
- [x] 4.4 Create `web/src/api/types.ts` — TypeScript types mirroring Go response shapes (Owner, Taxi, Driver, Expense, Report)
- [x] 4.5 Create `web/src/context/AuthContext.tsx` — token state, login(), logout(), isAuthenticated
- [x] 4.6 Create `web/src/router.tsx` — React Router setup with protected route wrapper (redirect to `/login` if no token)

## Phase 5: React Pages

- [x] 5.1 Create `web/src/pages/Login.tsx` — email + password form; POST `/auth/owner/login`; store token; redirect to `/`
- [x] 5.2 Create `web/src/pages/Dashboard.tsx` — fetch counts (active taxis, active drivers, confirmed expenses); display as cards
- [x] 5.3 Create `web/src/pages/Taxis.tsx` — list taxis table; inline create form (plate, model, year); handle 409 duplicate plate error
- [x] 5.4 Create `web/src/pages/Drivers.tsx` — list drivers table; create driver form; "Generate link" button → display `t.me` deep-link in Dialog
- [x] 5.5 Create `web/src/pages/Expenses.tsx` — table of confirmed expenses; Approve / Reject buttons; Reject opens Dialog for optional reason
- [ ] 5.6 Create `web/src/pages/ExpenseDetail.tsx` — receipt image, OCR fields, approve/reject controls
- [x] 5.7 Create `web/src/pages/Reports.tsx` — date-range picker; per-taxi and per-driver totals tables

## Phase 6: Integration & Deployment

- [x] 6.1 Create `cmd/web/main.go` — static file server embedding `web/dist/` on `WEB_PORT` (default 3000)
- [x] 6.2 Modify `Dockerfile` — add Node build stage (`node:20-alpine`): install deps, `npm run build`; copy `web/dist/` to final image
- [x] 6.3 Modify `docker-compose.yml` — add `web` service on port 3000; add `CORS_ORIGIN=http://localhost:3000` and `BOOTSTRAP_SECRET` to `api` service env
- [ ] 6.4 Add `BOOTSTRAP_SECRET` and `CORS_ORIGIN` to `.env.example` — blocked by permissions; create manually
