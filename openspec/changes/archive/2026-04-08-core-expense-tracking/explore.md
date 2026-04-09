## Exploration: core-expense-tracking

### Summary

gentax is a greenfield Go + PostgreSQL application for taxi expense management, with Telegram as the primary driver-facing interface. The core foundation requires decisions across six areas: project layout, database access, API style, authentication, data modeling, and OCR integration. The recommended path is `cmd/` + `internal/` layout with domain-driven packages, `sqlc` for type-safe database access, a lightweight REST API using `chi` (the Telegram bot calls the API internally, so gRPC overhead is not justified), Telegram user ID as the driver identity anchored by JWT tokens, and an async OCR pipeline that decouples receipt photo processing from expense confirmation. The data model treats `Receipt` as a prerequisite for a confirmed `Expense`, enforcing fraud prevention at the DB constraint level. Multi-tenancy is owner-scoped: every taxi and driver belongs to an owner, and all queries are filtered by owner ID.

---

### Decision Points

#### 1. Go Project Layout

**Options considered**:

| Layout | Description |
|--------|-------------|
| `cmd/` + `internal/` standard | Binary entry points in `cmd/`, all importable packages under `internal/` |
| Flat structure | All packages at root level, no enforced encapsulation |
| Domain-driven packages | Packages named after business concepts: `expense`, `driver`, `taxi`, `receipt` |
| Layer-driven packages | Packages named after technical layers: `handler`, `service`, `repository` |

**Recommendation**: `cmd/` + `internal/` layout with **domain-driven packages inside `internal/`**.

**Rationale**:
- `internal/` prevents accidental exposure of implementation packages — critical for a project that may grow a public SDK or CLI tooling later.
- Domain-driven packages (`internal/expense`, `internal/driver`, `internal/taxi`, `internal/receipt`) keep related types, service logic, and repository interfaces co-located. This makes each domain independently testable (a key TDD requirement): you mock `expense.Repository` without knowing anything about `driver`.
- Layer-driven packages (`handler/`, `service/`, `repository/`) scatter a single feature across three directories — hard to navigate and hard to mock in isolation.
- Flat structure provides no encapsulation and becomes unmaintainable past ~10 files.
- A Telegram bot binary and a potential admin API binary can coexist under `cmd/bot/` and `cmd/api/` sharing the same `internal/` packages — zero duplication.

**Proposed top-level structure**:
```
cmd/
  bot/         — Telegram bot entry point
  api/         — REST API entry point (admin / driver API)
internal/
  taxi/        — Taxi entity, service, repository interface
  driver/      — Driver entity, service, repository interface
  expense/     — Expense entity, service, repository interface
  receipt/     — Receipt entity, OCR integration point, repository interface
  auth/        — JWT issuance and validation
  db/          — DB connection, migrations, sqlc generated code
  telegram/    — Bot handler wiring
  httpapi/     — HTTP router, middleware, handlers
migrations/    — SQL migration files (goose or migrate)
openspec/      — SDD artifacts
```

---

#### 2. Database Access Layer

**Options considered**:

| Option | Type | Pro | Con |
|--------|------|-----|-----|
| `database/sql` + `pgx` | Raw driver | Full control, no magic | Boilerplate scan loops, error-prone |
| `sqlx` | Thin wrapper | Struct scanning, named params | Still manual SQL, no compile-time safety |
| `sqlc` | Code generator | Type-safe Go from SQL, compile-time errors | Requires SQL-first mindset, migration + query files as source of truth |
| GORM | Full ORM | Quick CRUD, auto-migrate | Magic, hard to test, N+1 traps, poor complex-query support |
| Bun | Full ORM + query builder | Better than GORM, query builder | Still ORM abstraction, less transparent |

**Recommendation**: **`sqlc`** with `pgx/v5` as the underlying driver.

**Rationale**:
- `sqlc` generates fully type-safe Go structs and query functions directly from `.sql` files. The generated code is ordinary Go — easy to read, easy to mock (generated interfaces), and plays perfectly with `testify/mock`.
- TDD alignment: `sqlc` generates a `Querier` interface alongside the implementation. Tests mock `Querier` — no real DB needed for unit tests. Integration tests hit a real Postgres container.
- The domain is SQL-heavy: expense aggregations by driver, date range, taxi, expense type. Writing these as clean SQL (not ORM chains) is both more readable and more performant.
- `pgx/v5` is the fastest Postgres driver for Go and supports `pgx`-native types (e.g., `pgtype.Numeric` for money) without the overhead of `database/sql` interface conversion.
- GORM is explicitly excluded: its magic conflicts with strict TDD (hard to assert what SQL is actually executed), and complex reporting queries require dropping to raw SQL anyway.

**Tooling**:
- Migrations: `golang-migrate` (simple, widely used, works with Makefile targets)
- `sqlc.yaml` configured with `emit_interface: true` and `emit_methods_with_db_argument: false`

---

#### 3. API Style

**Options considered**:

| Option | Description |
|--------|-------------|
| `net/http` stdlib + `chi` | Stdlib-compatible router, minimal dependencies |
| `gin` | Fast, popular, but non-stdlib `Context` |
| `echo` | Similar to gin, good middleware |
| gRPC | Proto-first, binary protocol, streaming |

**Recommendation**: **`net/http` + `chi`** router.

**Rationale**:
- The Telegram bot IS the primary client. The bot runs in the same process (or a sibling binary) and calls internal Go service functions directly — it does not call the REST API over the network. The REST API serves the admin dashboard and potential future mobile clients.
- `chi` is 100% compatible with `net/http` — middleware, handlers, and test utilities (`httptest.NewRecorder`) all work without adapters. This is the strongest TDD argument: `net/http/httptest` is the standard testing tool; gin/echo require framework-specific test helpers.
- `chi` provides route groups (v1 versioning), middleware chaining, and URL parameter extraction with zero magic.
- gRPC: justified only when you have many internal service-to-service calls at high RPC volume. Here, the bot calls Go functions directly; the admin client is a human browser. Proto overhead and code-gen complexity are not justified.
- `gin` and `echo` are fine but introduce non-stdlib `Context` types, complicating handler signatures and making them harder to test with the standard library alone.

---

#### 4. Authentication & Multi-tenancy

**Options considered**:

**Driver authentication**:
- Telegram user ID as identity (driver logs in via Telegram — Telegram guarantees the user ID is authentic)
- Username + password (traditional, but drivers never use the web)
- OAuth (overkill for a Telegram-native app)

**Multi-tenancy**:
- Tenant per taxi (taxi is the isolation unit)
- Tenant per owner (owner is the isolation unit; taxis and drivers are scoped under the owner)
- Separate schemas/databases per tenant (over-engineered for this scale)

**Auth tokens**:
- JWT (stateless, easy to verify in middleware)
- Server-side sessions (requires session store, more infra)

**Recommendation**:
- **Telegram user ID as driver identity** — the bot receives `update.Message.From.ID` (Telegram guarantees this). On first interaction, the bot registers the driver (or links their Telegram ID to a pre-created driver record). The bot exchanges the Telegram ID for a short-lived JWT.
- **Owner-scoped multi-tenancy** — every `Taxi` and `Driver` has an `owner_id` FK. All queries include `WHERE owner_id = $1`. This keeps the schema simple and allows one owner to manage many taxis/drivers without cross-contamination.
- **JWT for API calls** — short-lived (1h) access tokens, issued at bot start or when the admin logs in via the web. Claims include `user_id`, `role` (driver | admin), and `owner_id`. Middleware validates and injects a typed `Claims` struct into `context.Context`.

**Rationale**:
- Telegram user ID is cryptographically tied to the user's phone number — it cannot be spoofed in a legitimate bot interaction. This eliminates the need for a separate login flow for drivers.
- Owner-scoped tenancy is the simplest model that supports "multiple taxis from day 1" without requiring schema migrations when a new taxi is added. All filtering is in SQL `WHERE` clauses.
- JWT is stateless and trivially testable: tests create signed tokens with known claims and pass them as `Authorization: Bearer` headers.

---

#### 5. Data Model (draft)

**Entities and relationships**:

```
owners
  id            UUID        PK
  name          TEXT        NOT NULL
  email         TEXT        UNIQUE NOT NULL
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()

taxis
  id            UUID        PK
  owner_id      UUID        FK → owners.id NOT NULL
  plate         TEXT        NOT NULL
  model         TEXT
  year          INT
  active        BOOLEAN     NOT NULL DEFAULT true
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
  UNIQUE(owner_id, plate)

drivers
  id            UUID        PK
  owner_id      UUID        FK → owners.id NOT NULL
  telegram_id   BIGINT      UNIQUE NOT NULL   -- Telegram user ID
  full_name     TEXT        NOT NULL
  phone         TEXT
  active        BOOLEAN     NOT NULL DEFAULT true
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()

driver_taxi_assignments
  id            UUID        PK
  driver_id     UUID        FK → drivers.id NOT NULL
  taxi_id       UUID        FK → taxis.id NOT NULL
  assigned_at   TIMESTAMPTZ NOT NULL DEFAULT now()
  unassigned_at TIMESTAMPTZ                   -- NULL = currently active
  UNIQUE(driver_id, taxi_id, assigned_at)     -- history-safe

expense_categories
  id            UUID        PK
  owner_id      UUID        FK → owners.id NOT NULL
  name          TEXT        NOT NULL           -- "Fuel", "Repair", "Toll"
  UNIQUE(owner_id, name)

receipts
  id            UUID        PK
  driver_id     UUID        FK → drivers.id NOT NULL
  taxi_id       UUID        FK → taxis.id NOT NULL
  photo_url     TEXT        NOT NULL           -- stored in object storage (S3/GCS)
  telegram_file_id TEXT                        -- Telegram file_id for re-download
  ocr_status    TEXT        NOT NULL DEFAULT 'pending'  -- pending|processing|done|failed
  ocr_raw       JSONB                          -- raw OCR response
  extracted_amount   NUMERIC(12,2)
  extracted_date     DATE
  extracted_vendor   TEXT
  extracted_tax_id   TEXT                      -- RUC/RFC/NIT depending on country
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()

expenses
  id            UUID        PK
  owner_id      UUID        FK → owners.id NOT NULL
  driver_id     UUID        FK → drivers.id NOT NULL
  taxi_id       UUID        FK → taxis.id NOT NULL
  category_id   UUID        FK → expense_categories.id NOT NULL
  receipt_id    UUID        FK → receipts.id NOT NULL    -- REQUIRED (fraud prevention)
  amount        NUMERIC(12,2) NOT NULL
  expense_date  DATE        NOT NULL
  notes         TEXT
  status        TEXT        NOT NULL DEFAULT 'pending'   -- pending|approved|rejected
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Key design notes**:
- `receipt_id` on `expenses` is NOT NULL — you cannot create a confirmed expense without a receipt. This enforces fraud prevention at the DB constraint level, not just in application logic.
- `ocr_status` on `receipts` drives the async pipeline: the expense is created in `pending` status while OCR runs; once OCR completes successfully, the expense can be moved to `approved` (or flagged for manual review if amounts don't match).
- `driver_taxi_assignments` with `unassigned_at` preserves historical assignment data — important for audit trails on old expenses.
- `expense_categories` is owner-scoped so each fleet owner can customize their expense types.
- `extracted_tax_id` covers fiscal IDs across Latin American countries (RUC in Peru/Ecuador, RFC in Mexico, NIT in Colombia) — field name is generic.
- All monetary amounts use `NUMERIC(12,2)` — never `FLOAT` (precision loss).
- No hard deletes: `active` boolean on `taxis` and `drivers` for soft-delete. Expenses use `status` field, not deletion.
- UUIDs as PKs: avoids sequential ID enumeration, works across distributed inserts.

---

#### 6. OCR Integration Point

**Options considered**:

| Approach | Description |
|----------|-------------|
| Sync (blocking) | Bot waits for OCR to complete before responding to driver |
| Async (queue-based) | Bot uploads photo, creates `receipt` with `ocr_status=pending`, returns immediately; worker processes OCR and updates receipt |
| Hybrid | Async with bot notification when done |

**OCR provider options**:
- Google Cloud Vision API (high accuracy, cloud, costs per request)
- AWS Textract (similar to Vision, better for structured forms)
- Tesseract (open-source, self-hosted, lower accuracy on real photos)
- OpenAI Vision / GPT-4o (high accuracy, JSON-structured output possible, costs per request)

**Recommendation**: **Async pipeline with Telegram callback notification**.

**Rationale**:
- OCR can take 2–10 seconds depending on provider. Blocking the Telegram bot response for 10 seconds is a poor UX — Telegram's own timeout is 60s but the driver would see the bot as "frozen."
- The async flow: driver sends photo → bot uploads to storage → creates `Receipt{ocr_status: "pending"}` and `Expense{status: "pending"}` → responds "Receipt received, processing…" → background worker calls OCR API → updates `receipts.ocr_*` fields → sends bot notification to driver: "Receipt processed: $45.00 at PEMEX on 2026-04-05. Confirm?"
- The worker is a simple goroutine pool or a lightweight job queue (no need for Redis/Kafka at this scale — a `receipts` table poll with `FOR UPDATE SKIP LOCKED` is sufficient for a single-instance deployment).
- For the OCR provider: **Google Cloud Vision** is recommended as the primary option — it has the best handwriting + printed-receipt accuracy and a straightforward Go SDK. OpenAI GPT-4o Vision is a strong alternative for structured JSON extraction (can be prompted to return `{amount, date, vendor, tax_id}` directly).
- The `receipt` → `expense` link is set at receipt creation time with a DB FK. The expense `amount` starts as `extracted_amount` from OCR and can be corrected by the driver before final submission.

**Data fields to extract from receipt**:
```
amount       — total amount paid (required)
date         — transaction date (required)
vendor       — business name (required)
tax_id       — fiscal ID of the vendor (RUC/RFC/NIT) (optional but valuable)
currency     — ISO 4217 code, default to owner's configured currency
raw_text     — full OCR text (stored in ocr_raw JSONB for audit/reprocessing)
```

**Integration point in code** (`internal/receipt`):
```
receipt.Processor interface
  Process(ctx, receiptID UUID) error

receipt.OCRClient interface
  ExtractData(ctx, photoURL string) (OCRResult, error)
```

Both interfaces are injected — unit tests mock `OCRClient`; integration tests call a real provider with a test image.

---

### Open Questions

1. **Country / locale**: Which country's fiscal receipts are the primary target? Tax ID format (RUC/RFC/NIT) and currency matter for OCR prompt engineering and validation rules.
2. **OCR provider budget**: Google Vision costs ~$1.50 per 1,000 requests. At what volume does the owner operate? This affects provider choice (self-hosted Tesseract vs cloud API).
3. **Admin interface**: Is the Owner/Admin using a web dashboard, a separate Telegram bot, or just an API? This affects how much effort to put into the REST API vs bot commands.
4. **Driver onboarding flow**: Does the owner pre-create driver records (and then the driver links their Telegram account), or does the driver self-register via the bot?
5. **Expense approval workflow**: Does the owner manually approve every expense, or is auto-approval the default (with manual review only on OCR mismatches)?
6. **Deployment target**: Single VPS, Docker Compose, or cloud (GKE/ECS)? Affects job queue choice (in-process goroutines vs external queue).
7. **Currency handling**: Single currency per owner? Or can drivers submit expenses in different currencies (e.g., cross-border routes)?

---

### Risks

1. **OCR accuracy on real receipts**: Crumpled, poorly lit, or handwritten receipts degrade OCR significantly. The system must handle `ocr_status=failed` gracefully and allow drivers to enter amounts manually as a fallback.
2. **Telegram file availability**: Telegram file IDs expire after a period. Photos must be downloaded and stored in persistent object storage (S3/GCS) immediately when received — not referenced by Telegram file ID alone.
3. **Multi-taxi from day 1 complexity**: `owner_id` filtering in every query must be enforced consistently. Missing a `WHERE owner_id = $1` clause is a data-leakage bug. This should be caught by integration tests that seed two owners and verify isolation.
4. **sqlc migration discipline**: `sqlc` requires keeping `.sql` query files and migration files in sync. If a migration renames a column, every query using that column must be updated simultaneously. A Makefile target (`make generate`) should run `sqlc generate` + `go build` to catch breakage early.
5. **JWT secret management**: JWT signing keys must be rotated and stored securely (env var / secrets manager). Hardcoded secrets in config files are a security risk in a multi-driver production system.
6. **Strict TDD overhead at bootstrap**: With `sqlc`-generated interfaces and injected OCR clients, the test scaffolding (mocks, fixtures, test DB setup) is non-trivial. Budget time for test infrastructure before writing feature tests.
