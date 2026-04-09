# Change Proposal: core-expense-tracking

## Intent

gentax currently does not exist — there is no codebase, no database, no running service. This change delivers the complete foundation of the taxi expense management platform: a working system where fleet owners in Bogotá can register taxis and drivers, drivers can submit expense receipts via Telegram (with photos processed by OCR), and owners can review and manage everything through a web admin interface. Without this foundation, no subsequent feature (advanced reporting, exports, notifications) can be built. The goal is to go from zero to a deployable, end-to-end functional system with strict test coverage.

## Scope

### In Scope

- **Go project scaffolding**: `cmd/` + `internal/` layout with domain-driven packages, Go modules, Makefile, Docker Compose for local development (PostgreSQL + app).
- **Database schema and migrations**: All six core tables (`owners`, `taxis`, `drivers`, `driver_taxi_assignments`, `expense_categories`, `receipts`, `expenses`) via `golang-migrate`. sqlc query files and generated code.
- **Core domain entities and service interfaces**: Each domain package (`taxi`, `driver`, `expense`, `receipt`) exposes its entity types, a `Service` struct, and a `Repository` interface (backed by sqlc's generated `Querier`).
- **REST API**: Full CRUD for expenses, drivers, taxis, and expense categories. Reporting endpoints for expenses filtered by date range, driver, taxi, and category. Served via `net/http` + `chi`.
- **JWT authentication middleware**: Telegram user ID exchange for short-lived JWT (1h). Middleware extracts claims (`user_id`, `role`, `owner_id`) and injects into `context.Context`. Role-based access: `driver` vs `admin`.
- **Telegram bot**: Driver-facing UX for expense registration. Core flow: driver sends receipt photo, selects taxi and category, bot creates pending expense + receipt, confirms back. Commands: `/start` (link Telegram ID to driver record), `/expense` (new expense flow), `/status` (check pending expenses).
- **OCR async pipeline**: Background worker polls `receipts` with `ocr_status=pending`, calls OCR provider (Google Cloud Vision or GPT-4o Vision), extracts NIT, CUFE, total, date, and vendor name, updates receipt record, notifies driver via Telegram.
- **Admin web interface**: Basic web UI for the fleet owner. View and filter expenses, manage drivers and taxis, approve/reject pending expenses. Technology TBD in design phase (likely server-rendered Go templates or a lightweight SPA).
- **Test infrastructure**: testcontainers for PostgreSQL integration tests, testify/assert + testify/mock for unit tests, table-driven test patterns. All code written TDD-first.

### Out of Scope

- Advanced reporting and export (CSV, PDF generation)
- Push notifications beyond Telegram bot messages
- Mobile application (native iOS/Android)
- Multi-currency support (COP only, single currency per instance)
- Driver self-registration via Telegram bot (owner creates drivers manually)
- OAuth or third-party SSO (Telegram ID + JWT is the auth model)
- Multi-region deployment or horizontal scaling infrastructure
- Expense receipt storage redundancy (single object storage backend)

## Approach

### Architecture Pattern

The system follows a **modular monolith** pattern with two entry points (`cmd/bot` for Telegram, `cmd/api` for REST + admin web) sharing the same `internal/` domain packages. This avoids the complexity of microservices while keeping domain boundaries clean. The Telegram bot calls Go service functions directly (in-process) — it does not make HTTP calls to the REST API. The REST API serves the admin web interface and is the public contract for future clients.

### Package Responsibilities

| Package | Responsibility |
|---------|----------------|
| `cmd/bot` | Telegram bot entry point. Wires dependencies, starts long-polling. |
| `cmd/api` | REST API + admin web entry point. Wires dependencies, starts HTTP server. |
| `internal/taxi` | Taxi entity, service logic, repository interface. |
| `internal/driver` | Driver entity, service logic, repository interface. Telegram ID linking. |
| `internal/expense` | Expense entity, service logic (create, approve, reject, list with filters), repository interface. |
| `internal/receipt` | Receipt entity, OCR client interface, receipt processor, repository interface. |
| `internal/auth` | JWT issuance (`Sign`) and validation (`Verify`). Claims struct with `user_id`, `role`, `owner_id`. |
| `internal/db` | PostgreSQL connection pool (`pgxpool`), sqlc generated code, migration runner. |
| `internal/telegram` | Bot update handlers, conversation state management, callback query routing. |
| `internal/httpapi` | Chi router setup, middleware stack (auth, logging, CORS), REST handlers, admin web handlers. |
| `internal/worker` | OCR background processor. Polls `receipts` table with `FOR UPDATE SKIP LOCKED`, calls `receipt.OCRClient`, updates records, sends Telegram notifications. |

### Shared Domain Logic

The Telegram bot and REST API share domain logic through the service layer. For example, `expense.Service.Create()` is called both by `internal/telegram` (when a driver submits via bot) and by `internal/httpapi` (when an admin creates an expense via the web). Neither entry point contains business logic — all validation, authorization checks, and state transitions live in the service layer.

### OCR Integration

OCR is fully asynchronous and decoupled from both the bot and API:

1. Driver sends photo via Telegram (or admin uploads via web).
2. Photo is stored in object storage (GCS/S3). A `receipt` record is created with `ocr_status=pending`.
3. A linked `expense` record is created with `status=pending`.
4. The background worker (`internal/worker`) picks up pending receipts using `SELECT ... FOR UPDATE SKIP LOCKED` (no external queue needed at this scale).
5. Worker calls `receipt.OCRClient.ExtractData()` — extracts NIT del proveedor, CUFE, total, date, concept.
6. Worker updates the receipt with extracted fields and sets `ocr_status=done` (or `failed`).
7. Worker sends a Telegram message to the driver confirming the extracted data or requesting manual correction.

The `OCRClient` interface is injected, making the provider swappable and fully mockable in tests.

### Colombia-Specific Details

- All monetary amounts in COP using `NUMERIC(12,2)`.
- OCR extraction targets DIAN electronic invoice fields: NIT del proveedor, CUFE (Codigo Unico de Factura Electronica), total, date, concept.
- `extracted_tax_id` stores the vendor's NIT in DIAN standard format.
- Validation rules for NIT format (digits + check digit) will be implemented in the receipt processing pipeline.

## Rollback Plan

**Pre-production (current state)**: Rollback is trivial — delete the repository or reset to the initial commit. No data exists, no users are affected.

**Post-production rollback strategies**:
- **OCR pipeline failure**: Disable the background worker. Drivers fall back to manual expense entry (amount, date, vendor entered via bot text input instead of photo). The system remains functional without OCR.
- **Telegram bot failure**: The REST API and admin web continue to work. The owner can enter expenses manually via the admin interface until the bot is restored.
- **Database migration rollback**: Every migration has a corresponding `down` file. `golang-migrate` supports rollback to any version. Critical: test down migrations in CI before deploying up migrations.
- **Full rollback**: Stop services, restore database from backup, redeploy previous binary. Docker Compose makes this a single `docker compose down && docker compose up` with the previous image tag.

## Affected Modules

New Go packages to be created:

| Package | Type |
|---------|------|
| `cmd/bot/` | Binary entry point |
| `cmd/api/` | Binary entry point |
| `internal/taxi/` | Domain package |
| `internal/driver/` | Domain package |
| `internal/expense/` | Domain package |
| `internal/receipt/` | Domain package |
| `internal/auth/` | Domain package |
| `internal/db/` | Infrastructure package |
| `internal/telegram/` | Adapter package |
| `internal/httpapi/` | Adapter package |
| `internal/worker/` | Infrastructure package |
| `migrations/` | SQL migration files |

Supporting files:
- `go.mod`, `go.sum` — Go module definition
- `Makefile` — build, test, generate, migrate targets
- `docker-compose.yml` — local dev environment (PostgreSQL, app)
- `sqlc.yaml` — sqlc configuration
- `.env.example` — environment variable template

## Risks

### 1. OCR Accuracy on Real Colombian Receipts (HIGH)
Colombian receipts vary wildly: thermal paper that fades, handwritten notes, poor lighting in photos taken inside taxis. OCR extraction of NIT and CUFE may fail on a significant percentage of receipts. **Mitigation**: Always allow manual fallback entry. Store raw OCR response in `ocr_raw` JSONB for reprocessing with improved prompts. Track `ocr_status=failed` rate as an operational metric.

### 2. Telegram File Expiration (MEDIUM)
Telegram file IDs are not permanent. If the system stores only the `telegram_file_id` and attempts to download later, the file may be gone. **Mitigation**: Download and store the photo in persistent object storage (GCS/S3) immediately upon receipt. The `telegram_file_id` is kept only as a convenience reference, never as the source of truth.

### 3. Owner-Scoped Multi-tenancy Data Leakage (HIGH)
Every database query must include `WHERE owner_id = $1`. A single missing filter exposes one owner's data to another. This is the most dangerous class of bug in the system. **Mitigation**: All sqlc queries are reviewed for `owner_id` filtering. Integration tests seed two owners and verify strict isolation. The auth middleware injects `owner_id` from JWT claims — handlers never accept `owner_id` as a request parameter.

### 4. Strict TDD Bootstrap Overhead (MEDIUM)
Setting up the test infrastructure (testcontainers for PostgreSQL, mock generation for sqlc interfaces, test fixtures for each domain) is significant upfront work before any feature test can be written. **Mitigation**: Budget the first task batch entirely for test infrastructure. Use `sqlc`'s `emit_interface: true` to auto-generate mockable interfaces. Create shared test helpers (`internal/testutil/`) early.

### 5. JWT Secret Management in Production (MEDIUM)
Hardcoded or committed JWT signing keys are a critical security vulnerability in a system handling financial data. **Mitigation**: JWT secret loaded exclusively from environment variables. `.env` files are gitignored. Production deployment uses a secrets manager. Key rotation strategy documented before first production deploy.

## Success Criteria

1. **Driver expense submission via Telegram**: A driver can send a receipt photo via the Telegram bot, select a taxi and category, and see a pending expense created — verified by an integration test that exercises the full bot flow with a mocked Telegram API.

2. **OCR receipt processing**: A receipt photo submitted with `ocr_status=pending` is picked up by the background worker, processed through the OCR client, and updated with extracted NIT, CUFE, total, date, and vendor within 30 seconds — verified by an integration test with a real test image and mocked OCR client.

3. **Admin expense management via web**: An owner can log in to the admin web interface, view a list of expenses filtered by date range and driver, and approve or reject pending expenses — verified by HTTP handler tests using `httptest`.

4. **Multi-tenant isolation**: Two owners with separate drivers and taxis cannot see each other's data through any API endpoint or bot command — verified by integration tests that seed two owners and assert zero cross-contamination across all query paths.

5. **JWT authentication**: Unauthenticated requests to protected API endpoints return 401. Requests with an expired token return 401. Requests with a valid token for the wrong role return 403 — verified by table-driven middleware tests.

6. **Database schema integrity**: All migrations run cleanly up and down. The `receipt_id NOT NULL` constraint on `expenses` prevents expense creation without a receipt at the database level — verified by migration tests and constraint violation tests.

7. **Test coverage**: All domain service packages (`taxi`, `driver`, `expense`, `receipt`, `auth`) have unit test coverage above 80%. Integration tests cover the critical paths: expense creation, OCR processing, multi-tenant isolation, and authentication.

8. **Local development environment**: `docker compose up` followed by `make migrate` and `make run-bot` / `make run-api` starts a fully functional local system with PostgreSQL, both binaries, and seed data for one owner with two taxis and two drivers.
