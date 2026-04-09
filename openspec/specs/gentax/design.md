# Technical Design: core-expense-tracking

## 1. Package Structure

```
gentax/
├── cmd/
│   ├── api/
│   │   └── main.go          # REST API + admin web entry point; wires deps, starts HTTP server
│   └── bot/
│       └── main.go          # Telegram bot entry point; wires deps, starts long-polling
├── internal/
│   ├── auth/
│   │   ├── auth.go          # JWT issuance and validation; Claims struct
│   │   ├── auth_test.go
│   │   └── middleware.go    # Chi middleware: validates JWT, injects claims into context
│   ├── db/
│   │   ├── conn.go          # pgxpool connection factory; parses DATABASE_URL
│   │   ├── migrate.go       # golang-migrate runner; up/down helpers
│   │   └── query/           # sqlc-generated Go code (do not edit by hand)
│   │       ├── db.go
│   │       ├── models.go
│   │       ├── expenses.sql.go
│   │       ├── receipts.sql.go
│   │       ├── drivers.sql.go
│   │       ├── taxis.sql.go
│   │       └── owners.sql.go
│   ├── driver/
│   │   ├── driver.go        # Driver entity, Service struct, Repository interface
│   │   ├── driver_test.go
│   │   └── mock_repository.go  # testify/mock generated stub
│   ├── expense/
│   │   ├── expense.go       # Expense entity, Service struct, Repository interface
│   │   ├── expense_test.go
│   │   └── mock_repository.go
│   ├── httpapi/
│   │   ├── router.go        # Chi router setup; middleware stack (auth, logging, CORS)
│   │   ├── handlers/
│   │   │   ├── auth.go      # POST /auth/telegram
│   │   │   ├── taxis.go     # GET/POST /taxis, GET/PATCH /taxis/:id
│   │   │   ├── drivers.go   # GET/POST /drivers, PATCH /drivers/:id/assign/:taxiId
│   │   │   ├── expenses.go  # GET/POST /expenses, PATCH /expenses/:id/approve|reject
│   │   │   └── reports.go   # GET /reports/expenses
│   │   └── middleware/
│   │       ├── auth.go      # JWT extraction and context injection
│   │       ├── logging.go   # Structured request logging (slog)
│   │       └── cors.go      # CORS headers for admin web
│   ├── receipt/
│   │   ├── receipt.go       # Receipt entity, Processor, Repository interface, OCRClient interface
│   │   ├── receipt_test.go
│   │   ├── mock_ocr_client.go
│   │   └── mock_repository.go
│   ├── taxi/
│   │   ├── taxi.go          # Taxi entity, Service struct, Repository interface
│   │   ├── taxi_test.go
│   │   └── mock_repository.go
│   ├── telegram/
│   │   ├── bot.go           # Bot struct; update dispatcher; long-poll loop
│   │   ├── handlers.go      # /start, /expense, /status command handlers
│   │   ├── conversation.go  # In-memory FSM for multi-step expense flow
│   │   └── bot_test.go
│   ├── testutil/
│   │   ├── db.go            # testcontainers PostgreSQL fixture; RunMigrations helper
│   │   ├── fixtures.go      # Seed helpers: CreateOwner, CreateDriver, CreateTaxi
│   │   └── assert.go        # Domain-specific assertions (e.g. AssertExpenseEqual)
│   └── worker/
│       ├── ocr_worker.go    # Goroutine pool; polls receipts; calls OCRClient; updates DB
│       └── ocr_worker_test.go
├── migrations/
│   ├── 000001_create_owners.up.sql
│   ├── 000001_create_owners.down.sql
│   ├── 000002_create_taxis.up.sql
│   ├── 000002_create_taxis.down.sql
│   ├── 000003_create_drivers.up.sql
│   ├── 000003_create_drivers.down.sql
│   ├── 000004_create_driver_taxi_assignments.up.sql
│   ├── 000004_create_driver_taxi_assignments.down.sql
│   ├── 000005_create_expense_categories.up.sql
│   ├── 000005_create_expense_categories.down.sql
│   ├── 000006_create_receipts.up.sql
│   ├── 000006_create_receipts.down.sql
│   └── 000007_create_expenses.up.sql
│   └── 000007_create_expenses.down.sql
├── query/                   # sqlc SQL query source files (input to sqlc generate)
│   ├── expenses.sql
│   ├── receipts.sql
│   ├── drivers.sql
│   ├── taxis.sql
│   └── owners.sql
├── .env.example
├── docker-compose.yml
├── go.mod
├── go.sum
├── Makefile
└── sqlc.yaml
```

### Makefile Targets

```makefile
.PHONY: build test generate migrate-up migrate-down lint run-api run-bot

build:
	go build ./cmd/api ./cmd/bot

test:
	go test -race -cover ./...

test-unit:
	go test -race -cover -short ./...

test-integration:
	go test -race -cover -run Integration ./...

generate:
	sqlc generate
	go generate ./...          # runs mockery for interface stubs

migrate-up:
	go run ./cmd/api -migrate-up

migrate-down:
	go run ./cmd/api -migrate-down

lint:
	golangci-lint run ./...

run-api:
	go run ./cmd/api

run-bot:
	go run ./cmd/bot

docker-up:
	docker compose up -d

docker-down:
	docker compose down
```

### sqlc.yaml

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "query/"
    schema: "migrations/"
    gen:
      go:
        package: "query"
        out: "internal/db/query"
        emit_interface: true          # generates Querier interface for mocking
        emit_json_tags: true
        emit_db_tags: true
        null_style: "option"
```

---

## 2. Core Interfaces (Go)

### `internal/taxi`

```go
package taxi

import (
    "context"
    "time"

    "github.com/google/uuid"
)

// Taxi represents a vehicle registered to an owner.
type Taxi struct {
    ID           uuid.UUID
    OwnerID      uuid.UUID
    PlateNumber  string
    Model        string
    Year         int
    Active       bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// Repository defines the persistence contract for taxis.
// Backed by sqlc-generated Querier; owner_id filter is mandatory on every method.
type Repository interface {
    Create(ctx context.Context, ownerID uuid.UUID, plate, model string, year int) (*Taxi, error)
    GetByID(ctx context.Context, ownerID, taxiID uuid.UUID) (*Taxi, error)
    ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]*Taxi, error)
    Update(ctx context.Context, ownerID, taxiID uuid.UUID, model string, year int, active bool) (*Taxi, error)
    Delete(ctx context.Context, ownerID, taxiID uuid.UUID) error
}

// Service exposes business operations on taxis.
type Service interface {
    Register(ctx context.Context, ownerID uuid.UUID, plate, model string, year int) (*Taxi, error)
    Get(ctx context.Context, ownerID, taxiID uuid.UUID) (*Taxi, error)
    List(ctx context.Context, ownerID uuid.UUID) ([]*Taxi, error)
    Update(ctx context.Context, ownerID, taxiID uuid.UUID, model string, year int, active bool) (*Taxi, error)
    Deactivate(ctx context.Context, ownerID, taxiID uuid.UUID) error
}
```

### `internal/driver`

```go
package driver

import (
    "context"
    "time"

    "github.com/google/uuid"
)

// Driver represents a taxi driver linked to an owner.
type Driver struct {
    ID             uuid.UUID
    OwnerID        uuid.UUID
    FullName       string
    TelegramUserID int64     // 0 if not yet linked
    Active         bool
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// Repository defines the persistence contract for drivers.
type Repository interface {
    Create(ctx context.Context, ownerID uuid.UUID, fullName string) (*Driver, error)
    GetByID(ctx context.Context, ownerID, driverID uuid.UUID) (*Driver, error)
    GetByTelegramID(ctx context.Context, telegramUserID int64) (*Driver, error)
    ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]*Driver, error)
    LinkTelegram(ctx context.Context, ownerID, driverID uuid.UUID, telegramUserID int64) error
    AssignTaxi(ctx context.Context, ownerID, driverID, taxiID uuid.UUID) error
    UnassignTaxi(ctx context.Context, ownerID, driverID uuid.UUID) error
    Update(ctx context.Context, ownerID, driverID uuid.UUID, fullName string, active bool) (*Driver, error)
}

// Service exposes business operations on drivers.
type Service interface {
    Register(ctx context.Context, ownerID uuid.UUID, fullName string) (*Driver, error)
    Get(ctx context.Context, ownerID, driverID uuid.UUID) (*Driver, error)
    List(ctx context.Context, ownerID uuid.UUID) ([]*Driver, error)
    LinkTelegramID(ctx context.Context, driverID uuid.UUID, telegramUserID int64) error
    AssignToTaxi(ctx context.Context, ownerID, driverID, taxiID uuid.UUID) error
    Update(ctx context.Context, ownerID, driverID uuid.UUID, fullName string, active bool) (*Driver, error)
}
```

### `internal/expense`

```go
package expense

import (
    "context"
    "time"

    "github.com/google/uuid"
)

// Status represents the lifecycle state of an expense.
type Status string

const (
    StatusPending  Status = "pending"
    StatusApproved Status = "approved"
    StatusRejected Status = "rejected"
)

// Expense is a single expenditure tied to a receipt, driver, and taxi.
type Expense struct {
    ID           uuid.UUID
    OwnerID      uuid.UUID
    DriverID     uuid.UUID
    TaxiID       uuid.UUID
    ReceiptID    uuid.UUID  // NOT NULL — every expense requires a receipt
    CategoryID   uuid.UUID
    AmountCOP    float64    // NUMERIC(12,2) in COP
    Description  string
    ExpenseDate  time.Time
    Status       Status
    ReviewedBy   *uuid.UUID // admin user who approved/rejected; nil if pending
    ReviewedAt   *time.Time
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// ListFilter defines optional filters for listing expenses.
type ListFilter struct {
    TaxiID     *uuid.UUID
    DriverID   *uuid.UUID
    CategoryID *uuid.UUID
    Status     *Status
    From       *time.Time
    To         *time.Time
}

// Repository defines the persistence contract for expenses.
type Repository interface {
    Create(ctx context.Context, ownerID, driverID, taxiID, receiptID, categoryID uuid.UUID, amountCOP float64, description string, expenseDate time.Time) (*Expense, error)
    GetByID(ctx context.Context, ownerID, expenseID uuid.UUID) (*Expense, error)
    List(ctx context.Context, ownerID uuid.UUID, filter ListFilter) ([]*Expense, error)
    Approve(ctx context.Context, ownerID, expenseID, reviewerID uuid.UUID) (*Expense, error)
    Reject(ctx context.Context, ownerID, expenseID, reviewerID uuid.UUID) (*Expense, error)
    Delete(ctx context.Context, ownerID, expenseID uuid.UUID) error
}

// Service exposes business operations on expenses.
type Service interface {
    Create(ctx context.Context, ownerID, driverID, taxiID, receiptID, categoryID uuid.UUID, amountCOP float64, description string, expenseDate time.Time) (*Expense, error)
    Get(ctx context.Context, ownerID, expenseID uuid.UUID) (*Expense, error)
    List(ctx context.Context, ownerID uuid.UUID, filter ListFilter) ([]*Expense, error)
    Approve(ctx context.Context, ownerID, expenseID, reviewerID uuid.UUID) (*Expense, error)
    Reject(ctx context.Context, ownerID, expenseID, reviewerID uuid.UUID) (*Expense, error)
}
```

### `internal/receipt`

```go
package receipt

import (
    "context"
    "time"

    "github.com/google/uuid"
)

// OCRStatus represents the OCR processing lifecycle.
type OCRStatus string

const (
    OCRStatusPending OCRStatus = "pending"
    OCRStatusDone    OCRStatus = "done"
    OCRStatusFailed  OCRStatus = "failed"
)

// Receipt is the record of a physical receipt photo and its extracted DIAN fields.
type Receipt struct {
    ID                 uuid.UUID
    OwnerID            uuid.UUID
    DriverID           uuid.UUID
    StorageURL         string     // GCS/S3 URL — permanent storage
    TelegramFileID     string     // convenience ref only, not source of truth
    OCRStatus          OCRStatus
    OCRRetryCount      int
    OCRRaw             []byte     // raw JSON response from OCR provider
    ExtractedNIT       string     // NIT del proveedor (DIAN format)
    ExtractedCUFE      string     // Código Único de Factura Electrónica
    ExtractedTotal     *float64   // amount in COP
    ExtractedDate      *time.Time
    ExtractedConcepto  string     // vendor/concept name
    CreatedAt          time.Time
    UpdatedAt          time.Time
}

// ExtractedData holds structured OCR output from a receipt.
type ExtractedData struct {
    NIT       string   // validated DIAN NIT format: digits + check digit
    CUFE      string
    TotalCOP  float64
    Date      time.Time
    Concepto  string
    RawJSON   []byte   // full provider response for reprocessing
}

// Repository defines the persistence contract for receipts.
type Repository interface {
    Create(ctx context.Context, ownerID, driverID uuid.UUID, storageURL, telegramFileID string) (*Receipt, error)
    GetByID(ctx context.Context, ownerID, receiptID uuid.UUID) (*Receipt, error)
    ListPendingOCR(ctx context.Context, limit int) ([]*Receipt, error)  // FOR UPDATE SKIP LOCKED
    UpdateOCRResult(ctx context.Context, receiptID uuid.UUID, data ExtractedData) error
    MarkOCRFailed(ctx context.Context, receiptID uuid.UUID, retryCount int, rawResponse []byte) error
}

// OCRClient is the interface for OCR provider integrations (Google Vision, GPT-4o, etc.).
// Inject this to make provider swappable and fully mockable in tests.
type OCRClient interface {
    ExtractData(ctx context.Context, imageURL string) (*ExtractedData, error)
}

// Processor orchestrates receipt download, storage upload, and OCR queuing.
type Processor interface {
    Process(ctx context.Context, ownerID, driverID uuid.UUID, telegramFileID string) (*Receipt, error)
}
```

### `internal/auth`

```go
package auth

import (
    "context"
    "time"

    "github.com/google/uuid"
)

// Role controls access level within the system.
type Role string

const (
    RoleDriver Role = "driver"
    RoleAdmin  Role = "admin"
)

// Claims are the JWT payload fields injected into every authenticated request context.
type Claims struct {
    UserID    uuid.UUID // driver ID or admin user ID
    OwnerID   uuid.UUID // fleet owner — all data is scoped to this
    Role      Role
    ExpiresAt time.Time
}

// contextKey is the unexported key for storing Claims in context.Context.
type contextKey struct{}

// ClaimsKey is the key used to inject and extract Claims from context.
var ClaimsKey = contextKey{}

// TokenIssuer creates signed JWTs.
type TokenIssuer interface {
    // Issue signs a new JWT for the given claims. Returns the signed token string.
    Issue(claims Claims) (string, error)
}

// TokenValidator parses and validates JWT strings.
type TokenValidator interface {
    // Validate parses the token, checks signature and expiry, returns claims.
    Validate(ctx context.Context, token string) (*Claims, error)
}
```

---

## 3. Database Schema

```sql
-- ============================================================
-- owners
-- ============================================================
CREATE TABLE owners (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name   TEXT        NOT NULL,
    email       TEXT        NOT NULL UNIQUE,
    active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE owners IS 'Fleet owners. Each owner is a separate tenant; all data is owner_id-scoped.';

-- ============================================================
-- taxis
-- ============================================================
CREATE TABLE taxis (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id     UUID        NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    plate_number TEXT        NOT NULL,
    model        TEXT        NOT NULL,
    year         SMALLINT    NOT NULL CHECK (year >= 1990 AND year <= 2100),
    active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, plate_number)
);

CREATE INDEX idx_taxis_owner_id ON taxis (owner_id);

COMMENT ON TABLE taxis IS 'Vehicles registered per owner. plate_number is unique per owner (not globally).';

-- ============================================================
-- drivers
-- ============================================================
CREATE TABLE drivers (
    id                UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id          UUID        NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    full_name         TEXT        NOT NULL,
    telegram_user_id  BIGINT      UNIQUE,   -- NULL until driver links via /start
    active            BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_drivers_owner_id        ON drivers (owner_id);
CREATE INDEX idx_drivers_telegram_user_id ON drivers (telegram_user_id) WHERE telegram_user_id IS NOT NULL;

COMMENT ON TABLE drivers IS 'Drivers belong to one owner. telegram_user_id links bot identity to driver record.';

-- ============================================================
-- driver_taxi_assignments
-- ============================================================
CREATE TABLE driver_taxi_assignments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID        NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    driver_id   UUID        NOT NULL REFERENCES drivers(id) ON DELETE CASCADE,
    taxi_id     UUID        NOT NULL REFERENCES taxis(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    unassigned_at TIMESTAMPTZ,  -- NULL means currently assigned
    CONSTRAINT fk_driver_owner CHECK (true),  -- enforced via app layer: driver.owner_id = owner_id
    CONSTRAINT fk_taxi_owner   CHECK (true)   -- enforced via app layer: taxi.owner_id   = owner_id
);

CREATE INDEX idx_dta_driver_id ON driver_taxi_assignments (driver_id);
CREATE INDEX idx_dta_taxi_id   ON driver_taxi_assignments (taxi_id);
CREATE INDEX idx_dta_active    ON driver_taxi_assignments (driver_id) WHERE unassigned_at IS NULL;

COMMENT ON TABLE driver_taxi_assignments IS 'Tracks which driver is assigned to which taxi. unassigned_at=NULL means currently active.';

-- ============================================================
-- expense_categories
-- ============================================================
CREATE TABLE expense_categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID        NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    description TEXT,
    active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, name)
);

CREATE INDEX idx_expense_categories_owner_id ON expense_categories (owner_id);

COMMENT ON TABLE expense_categories IS 'Custom expense categories per owner (e.g. Combustible, Mantenimiento, Peaje).';

-- ============================================================
-- receipts
-- ============================================================
CREATE TABLE receipts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id            UUID        NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    driver_id           UUID        NOT NULL REFERENCES drivers(id),
    storage_url         TEXT        NOT NULL,       -- GCS/S3 permanent URL
    telegram_file_id    TEXT,                        -- Telegram convenience ref; NOT source of truth
    ocr_status          TEXT        NOT NULL DEFAULT 'pending'
                            CHECK (ocr_status IN ('pending', 'done', 'failed')),
    ocr_retry_count     SMALLINT    NOT NULL DEFAULT 0,
    ocr_raw             JSONB,                       -- raw OCR provider response for reprocessing
    extracted_nit       TEXT,                        -- NIT del proveedor (DIAN format: digits + check digit)
    extracted_cufe      TEXT,                        -- Código Único de Factura Electrónica
    extracted_total     NUMERIC(12,2),               -- amount in COP
    extracted_date      DATE,
    extracted_concepto  TEXT,                        -- vendor / concept
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_receipts_owner_id   ON receipts (owner_id);
CREATE INDEX idx_receipts_driver_id  ON receipts (driver_id);
CREATE INDEX idx_receipts_ocr_status ON receipts (ocr_status) WHERE ocr_status = 'pending';

COMMENT ON TABLE receipts IS 'Receipt photos. OCR fields populated asynchronously by the background worker.';
COMMENT ON COLUMN receipts.extracted_nit IS 'DIAN NIT format: numeric digits ending with check digit separated by hyphen, e.g. 900123456-7';
COMMENT ON COLUMN receipts.extracted_cufe IS 'CUFE: 96-character hex string unique per DIAN electronic invoice';

-- ============================================================
-- expenses
-- ============================================================
CREATE TABLE expenses (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id     UUID          NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    driver_id    UUID          NOT NULL REFERENCES drivers(id),
    taxi_id      UUID          NOT NULL REFERENCES taxis(id),
    receipt_id   UUID          NOT NULL REFERENCES receipts(id),   -- NOT NULL: fraud prevention
    category_id  UUID          NOT NULL REFERENCES expense_categories(id),
    amount_cop   NUMERIC(12,2) NOT NULL CHECK (amount_cop > 0),
    description  TEXT,
    expense_date DATE          NOT NULL,
    status       TEXT          NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by  UUID          REFERENCES owners(id),  -- admin who approved/rejected
    reviewed_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_review_consistency
        CHECK (
            (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL) OR
            (status IN ('approved', 'rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
        )
);

CREATE INDEX idx_expenses_owner_id    ON expenses (owner_id);
CREATE INDEX idx_expenses_driver_id   ON expenses (driver_id);
CREATE INDEX idx_expenses_taxi_id     ON expenses (taxi_id);
CREATE INDEX idx_expenses_status      ON expenses (owner_id, status);
CREATE INDEX idx_expenses_date        ON expenses (owner_id, expense_date DESC);
CREATE INDEX idx_expenses_category    ON expenses (owner_id, category_id);

COMMENT ON TABLE expenses IS 'Core expense records. receipt_id NOT NULL enforces that every expense has a supporting receipt at DB level.';
COMMENT ON COLUMN expenses.amount_cop IS 'Amount in Colombian Pesos (COP). NUMERIC(12,2) supports up to 9,999,999,999.99 COP.';
```

---

## 4. Key Flows

### 4a. Driver Submits Expense via Telegram

```
Driver (Telegram)
    │
    │  sends photo message
    ▼
internal/telegram.Bot.handlePhoto()
    │  calls receipt.Processor.Process(ctx, ownerID, driverID, telegramFileID)
    ▼
internal/receipt.Processor
    │  1. Downloads photo bytes from Telegram file API
    │  2. Uploads to GCS/S3 → storageURL
    │  3. Calls receipt.Repository.Create(ctx, ownerID, driverID, storageURL, telegramFileID)
    │  Returns *Receipt{ID, ocr_status:"pending"}
    ▼
internal/telegram.Bot (conversation FSM)
    │  prompts driver: "Select taxi" (inline keyboard)
    │  driver selects taxi → prompts "Select category" → driver selects
    │  calls expense.Service.Create(ctx, ownerID, driverID, taxiID, receipt.ID, categoryID, 0, "", today)
    │  (amount=0, will be updated after OCR completes)
    ▼
internal/expense.Service.Create()
    │  validates ownerID matches driver.OwnerID and taxi.OwnerID
    │  calls expense.Repository.Create(...)
    │  Returns *Expense{ID, status:"pending"}
    ▼
internal/telegram.Bot
    │  responds: "Gasto registrado. Procesando recibo... te aviso cuando esté listo."
    │
    ▼ (async — background goroutine pool)
    │
internal/worker.OCRWorker.poll()
    │  SELECT id, storage_url FROM receipts
    │  WHERE ocr_status = 'pending' FOR UPDATE SKIP LOCKED LIMIT 5
    │  Calls receipt.OCRClient.ExtractData(ctx, receipt.StorageURL)
    ▼
OCR Provider (Google Vision / GPT-4o)
    │  Returns NIT, CUFE, total, date, concepto
    ▼
internal/worker.OCRWorker
    │  Validates NIT format (DIAN check digit)
    │  Calls receipt.Repository.UpdateOCRResult(ctx, receiptID, extractedData)
    │  Updates linked expense amount_cop if extracted_total present
    │  Sends Telegram message to driver:
    │    "Recibo procesado: $45,000 COP — Combustible — NIT 900123456-7"
    ▼
Driver (Telegram)
    │  receives confirmation with extracted data
    │  can reply "Correcto" or "Corregir" to trigger manual entry
```

### 4b. Owner Approves Expense via Admin API

```
Admin Browser
    │
    │  PATCH /expenses/{id}/approve  (Bearer JWT)
    ▼
internal/httpapi/middleware.Auth
    │  Validates JWT → extracts Claims{UserID, OwnerID, Role:"admin"}
    │  Injects into context.Context
    ▼
internal/httpapi/handlers.ExpenseHandler.Approve()
    │  Reads Claims from context (never trusts request body for owner_id)
    │  Validates expense belongs to Claims.OwnerID
    ▼
internal/expense.Service.Approve(ctx, ownerID, expenseID, reviewerID)
    │  Checks current status == "pending" (rejects double-approve)
    │  Calls expense.Repository.Approve(ctx, ownerID, expenseID, reviewerID)
    ▼
internal/db.query (sqlc)
    │  UPDATE expenses SET status='approved', reviewed_by=$3, reviewed_at=NOW()
    │  WHERE id=$1 AND owner_id=$2 AND status='pending'
    │  Returns updated *Expense
    ▼
internal/expense.Service
    │  Fetches driver record to get telegram_user_id
    │  Calls internal/telegram.Bot.SendMessage(driverTelegramID, "Tu gasto fue aprobado.")
    ▼
Admin Browser
    │  receives 200 OK with updated Expense JSON
```

### 4c. JWT Authentication Flow

```
Driver (Telegram /start command)
    │
    │  /start command received by Bot
    ▼
internal/telegram.Bot.handleStart()
    │  Extracts Telegram user ID from update.Message.From.ID
    │  Calls driver.Repository.GetByTelegramID(ctx, telegramUserID)
    │    → If not found: replies "No estás registrado. Contacta a tu propietario."
    │    → If found: proceeds
    ▼
internal/auth.TokenIssuer.Issue(Claims{
    UserID:  driver.ID,
    OwnerID: driver.OwnerID,
    Role:    RoleDriver,
    ExpiresAt: now + 1h,
})
    │  Signs JWT with HS256 + JWT_SECRET
    │  Returns signed token string
    ▼
internal/telegram.Bot
    │  Stores token in in-memory conversation state (keyed by Telegram user ID)
    │  All subsequent bot service calls attach the token to context:
    │    ctx = context.WithValue(ctx, auth.ClaimsKey, claims)
    │
    │  (Bot calls services directly in-process — no HTTP round-trip)
    │
    ▼ REST API path (Admin)
    │
Admin → POST /auth/telegram  { telegram_user_id: 12345, owner_id: "..." }
    │  (owner provides telegram_user_id of their own account for admin access)
    ▼
internal/httpapi/handlers.AuthHandler.TelegramAuth()
    │  Looks up owner by telegram_user_id (or validates pre-shared admin token)
    │  Calls auth.TokenIssuer.Issue(Claims{..., Role: RoleAdmin})
    │  Returns { token: "eyJ..." }
    ▼
Admin → subsequent requests: Authorization: Bearer eyJ...
    ▼
internal/httpapi/middleware.Auth
    │  Calls auth.TokenValidator.Validate(ctx, bearerToken)
    │  On failure: 401 Unauthorized
    │  On role mismatch: 403 Forbidden
    │  On success: injects Claims into context, calls next handler
```

---

## 5. REST API Endpoints

### Request/Response Types

```go
// auth
type TelegramAuthRequest  struct { TelegramUserID int64  `json:"telegram_user_id"` }
type TelegramAuthResponse struct { Token string `json:"token"`; ExpiresAt time.Time `json:"expires_at"` }

// taxis
type CreateTaxiRequest  struct { PlateNumber string `json:"plate_number"`; Model string `json:"model"`; Year int `json:"year"` }
type TaxiResponse       struct { /* mirrors taxi.Taxi */ }

// drivers
type CreateDriverRequest struct { FullName string `json:"full_name"` }
type DriverResponse      struct { /* mirrors driver.Driver */ }
type AssignTaxiRequest   struct { /* no body — taxiId is in the URL */ }

// expenses
type CreateExpenseRequest struct {
    DriverID    string  `json:"driver_id"`
    TaxiID      string  `json:"taxi_id"`
    ReceiptID   string  `json:"receipt_id"`
    CategoryID  string  `json:"category_id"`
    AmountCOP   float64 `json:"amount_cop"`
    Description string  `json:"description"`
    ExpenseDate string  `json:"expense_date"` // YYYY-MM-DD
}
type ExpenseResponse struct { /* mirrors expense.Expense */ }
type ReviewRequest   struct { /* empty body — action is in the URL */ }

// reports
type ExpenseReportRequest struct {
    TaxiID     string `query:"taxi_id"`
    DriverID   string `query:"driver_id"`
    CategoryID string `query:"category_id"`
    Status     string `query:"status"`
    From       string `query:"from"` // YYYY-MM-DD
    To         string `query:"to"`   // YYYY-MM-DD
}
type ExpenseReportResponse struct {
    Expenses   []*ExpenseResponse `json:"expenses"`
    TotalCOP   float64            `json:"total_cop"`
    Count      int                `json:"count"`
}
```

### Endpoint Table

| Method | Path | Handler | Auth Required | Role |
|--------|------|---------|---------------|------|
| `POST` | `/auth/telegram` | `AuthHandler.TelegramAuth` | No | — |
| `GET` | `/taxis` | `TaxiHandler.List` | Yes | admin |
| `POST` | `/taxis` | `TaxiHandler.Create` | Yes | admin |
| `GET` | `/taxis/:id` | `TaxiHandler.Get` | Yes | admin |
| `PATCH` | `/taxis/:id` | `TaxiHandler.Update` | Yes | admin |
| `GET` | `/drivers` | `DriverHandler.List` | Yes | admin |
| `POST` | `/drivers` | `DriverHandler.Create` | Yes | admin |
| `GET` | `/drivers/:id` | `DriverHandler.Get` | Yes | admin |
| `PATCH` | `/drivers/:id` | `DriverHandler.Update` | Yes | admin |
| `POST` | `/drivers/:id/assign/:taxiId` | `DriverHandler.AssignTaxi` | Yes | admin |
| `DELETE` | `/drivers/:id/assign` | `DriverHandler.UnassignTaxi` | Yes | admin |
| `GET` | `/expenses` | `ExpenseHandler.List` | Yes | admin |
| `POST` | `/expenses` | `ExpenseHandler.Create` | Yes | admin, driver |
| `GET` | `/expenses/:id` | `ExpenseHandler.Get` | Yes | admin, driver |
| `PATCH` | `/expenses/:id/approve` | `ExpenseHandler.Approve` | Yes | admin |
| `PATCH` | `/expenses/:id/reject` | `ExpenseHandler.Reject` | Yes | admin |
| `GET` | `/reports/expenses` | `ReportHandler.Expenses` | Yes | admin |

All protected routes require `Authorization: Bearer <token>`. The `owner_id` is **never** accepted from the request body or query params — it is always extracted from the JWT claims.

---

## 6. OCR Worker Design

### Goroutine Pool and Polling

```go
// internal/worker/ocr_worker.go

const (
    workerPoolSize   = 3               // concurrent OCR calls
    pollInterval     = 5 * time.Second
    maxOCRRetries    = 3
    pollBatchSize    = 5               // LIMIT per poll cycle
)

type OCRWorker struct {
    receiptRepo receipt.Repository
    expenseRepo expense.Repository
    ocrClient   receipt.OCRClient
    botNotifier TelegramNotifier  // interface: SendMessage(telegramUserID int64, text string) error
    logger      *slog.Logger
}

func (w *OCRWorker) Run(ctx context.Context) {
    sem := make(chan struct{}, workerPoolSize)
    ticker := time.NewTicker(pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            w.processBatch(ctx, sem)
        }
    }
}

func (w *OCRWorker) processBatch(ctx context.Context, sem chan struct{}) {
    receipts, err := w.receiptRepo.ListPendingOCR(ctx, pollBatchSize)
    if err != nil { /* log, continue */ return }

    for _, r := range receipts {
        sem <- struct{}{}
        go func(r *receipt.Receipt) {
            defer func() { <-sem }()
            w.processOne(ctx, r)
        }(r)
    }
}
```

### SQL Query: Polling with SKIP LOCKED

```sql
-- query/receipts.sql

-- name: ListPendingOCR :many
SELECT id, owner_id, driver_id, storage_url, ocr_retry_count
FROM receipts
WHERE ocr_status = 'pending'
  AND ocr_retry_count < @max_retries
FOR UPDATE SKIP LOCKED
LIMIT @limit;
```

`FOR UPDATE SKIP LOCKED` ensures multiple worker pods never race to process the same receipt. At current scale (single pod) this is defensive; it costs nothing and avoids a class of duplicate-processing bugs as the system grows.

### Error Handling and Retry Logic

```go
func (w *OCRWorker) processOne(ctx context.Context, r *receipt.Receipt) {
    data, err := w.ocrClient.ExtractData(ctx, r.StorageURL)
    if err != nil {
        newCount := r.OCRRetryCount + 1
        if newCount >= maxOCRRetries {
            // Mark permanently failed; store raw error for manual review
            w.receiptRepo.MarkOCRFailed(ctx, r.ID, newCount, []byte(err.Error()))
            w.notifyDriver(ctx, r, "No pudimos procesar tu recibo automáticamente. Por favor corrígelo manualmente.")
            return
        }
        // Increment retry count; leave status = 'pending' so it gets picked up again
        w.receiptRepo.IncrementRetryCount(ctx, r.ID, newCount)
        return
    }

    if err := validateNIT(data.NIT); err != nil {
        w.logger.Warn("invalid NIT extracted", "receipt_id", r.ID, "nit", data.NIT)
        data.NIT = ""  // store empty; driver will correct
    }

    w.receiptRepo.UpdateOCRResult(ctx, r.ID, *data)
    w.notifyDriver(ctx, r, formatOCRConfirmation(data))
}
```

### DIAN NIT Validation

Colombian NIT format: `XXXXXXXXX-D` where `XXXXXXXXX` is a 9-digit base number and `D` is the check digit computed via a weighting algorithm defined by DIAN (weights: 3,7,13,17,19,23,29,37,41,43 applied right-to-left, mod 11).

```go
// internal/receipt/nit.go

// ValidateNIT checks Colombian DIAN NIT format.
// Accepts "9001234567" (no hyphen) or "900123456-7" (with hyphen).
// Returns error if format is invalid or check digit does not match.
func ValidateNIT(raw string) error {
    // Strip hyphen if present
    s := strings.ReplaceAll(raw, "-", "")
    if len(s) < 2 || len(s) > 10 {
        return fmt.Errorf("NIT length invalid: %q", raw)
    }
    for _, c := range s {
        if c < '0' || c > '9' {
            return fmt.Errorf("NIT must be numeric: %q", raw)
        }
    }
    base   := s[:len(s)-1]
    checkD := int(s[len(s)-1] - '0')

    weights := []int{3, 7, 13, 17, 19, 23, 29, 37, 41, 43}
    sum := 0
    for i, d := range reverse(base) {
        if i >= len(weights) { break }
        sum += int(d-'0') * weights[i]
    }
    remainder := sum % 11
    expected := 0
    if remainder >= 2 { expected = 11 - remainder }

    if checkD != expected {
        return fmt.Errorf("NIT check digit mismatch: got %d, expected %d", checkD, expected)
    }
    return nil
}
```

---

## 7. Test Strategy

Strict TDD Mode is enabled. All service logic is written test-first; no feature code is committed without a failing test that it makes pass.

### Unit Tests — Service Layer

Each service is tested with mocked repositories and OCR clients. Use `testify/mock`.

```go
// internal/expense/expense_test.go — example table-driven service test

func TestExpenseService_Create(t *testing.T) {
    tests := []struct {
        name      string
        setup     func(repo *MockRepository)
        wantErr   bool
        wantStatus expense.Status
    }{
        {
            name: "creates pending expense with valid receipt",
            setup: func(repo *MockRepository) {
                repo.On("Create", mock.Anything, ...).
                    Return(&expense.Expense{Status: expense.StatusPending}, nil)
            },
            wantStatus: expense.StatusPending,
        },
        {
            name: "fails when owner_id mismatches driver owner",
            setup: func(repo *MockRepository) { /* no repo call expected */ },
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := new(MockRepository)
            tt.setup(repo)
            svc := NewService(repo)
            exp, err := svc.Create(context.Background(), ...)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.wantStatus, exp.Status)
            repo.AssertExpectations(t)
        })
    }
}
```

### Integration Tests — Repository Layer

Use `testcontainers-go` to spin up a real PostgreSQL instance. Run all migrations up. Tests exercise sqlc-generated queries against a real database.

```go
// internal/testutil/db.go

func NewTestDB(t *testing.T) *pgxpool.Pool {
    t.Helper()
    ctx := context.Background()

    container, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:16-alpine"),
        postgres.WithDatabase("gentax_test"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
    )
    require.NoError(t, err)
    t.Cleanup(func() { container.Terminate(ctx) })

    connStr, err := container.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    RunMigrations(t, connStr)  // applies all up migrations

    pool, err := pgxpool.New(ctx, connStr)
    require.NoError(t, err)
    t.Cleanup(pool.Close)
    return pool
}
```

Integration tests are tagged with `//go:build integration` and run separately via `make test-integration`.

### Multi-Tenant Isolation Tests

A dedicated test file (`internal/testutil/isolation_test.go`) seeds **two owners** with separate taxis, drivers, and expenses, then asserts that all List/Get queries return zero cross-contamination.

```go
func TestMultiTenantIsolation(t *testing.T) {
    pool := testutil.NewTestDB(t)
    ownerA := testutil.CreateOwner(t, pool, "Owner A")
    ownerB := testutil.CreateOwner(t, pool, "Owner B")
    taxiA  := testutil.CreateTaxi(t, pool, ownerA.ID)
    taxiB  := testutil.CreateTaxi(t, pool, ownerB.ID)
    // ... seed expenses for each owner

    repo := expense.NewRepository(pool)
    expensesA, _ := repo.List(ctx, ownerA.ID, expense.ListFilter{})
    for _, e := range expensesA {
        assert.Equal(t, ownerA.ID, e.OwnerID, "owner A should only see their own expenses")
    }
    expensesB, _ := repo.List(ctx, ownerB.ID, expense.ListFilter{})
    for _, e := range expensesB {
        assert.Equal(t, ownerB.ID, e.OwnerID, "owner B should only see their own expenses")
    }
}
```

### Bot Handler Tests

Mock `expense.Service`, `receipt.Processor`, and `driver.Service`. Feed synthetic `tgbotapi.Update` objects and assert sent messages.

```go
// internal/telegram/handlers_test.go
func TestHandlePhoto_CreatesReceiptAndExpense(t *testing.T) {
    mockReceiptProc := new(receipt.MockProcessor)
    mockExpenseSvc  := new(expense.MockService)

    mockReceiptProc.On("Process", mock.Anything, ownerID, driverID, "file123").
        Return(&receipt.Receipt{ID: receiptID}, nil)
    mockExpenseSvc.On("Create", mock.Anything, ...).
        Return(&expense.Expense{ID: expenseID, Status: expense.StatusPending}, nil)

    bot := NewBot(mockReceiptProc, mockExpenseSvc, ...)
    update := buildPhotoUpdate(driverTelegramID, "file123")
    bot.handleUpdate(context.Background(), update)

    mockReceiptProc.AssertExpectations(t)
    mockExpenseSvc.AssertExpectations(t)
}
```

### Coverage Target

| Package | Target |
|---------|--------|
| `internal/auth` | 90% |
| `internal/expense` | 85% |
| `internal/driver` | 80% |
| `internal/taxi` | 80% |
| `internal/receipt` | 80% |
| `internal/worker` | 80% |
| `internal/httpapi` | 75% (handlers + middleware) |
| `internal/telegram` | 70% (conversation FSM) |

Run `go test -cover ./internal/...` in CI; fail the build if any package drops below its target.

---

## 8. Environment Configuration

All configuration is loaded exclusively from environment variables. No config files are committed with secrets. The `.env.example` is committed; `.env` is gitignored.

```bash
# .env.example

# ── Database ──────────────────────────────────────────────────────────────────
DATABASE_URL=postgres://gentax:password@localhost:5432/gentax?sslmode=disable

# ── Telegram ──────────────────────────────────────────────────────────────────
TELEGRAM_BOT_TOKEN=123456789:AAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# ── JWT ───────────────────────────────────────────────────────────────────────
# Minimum 32 bytes of entropy. Rotate before production.
JWT_SECRET=changeme-use-a-long-random-secret-in-production

# ── OCR ───────────────────────────────────────────────────────────────────────
# OCR_PROVIDER selects which client implementation is wired at startup.
OCR_PROVIDER=google_vision    # options: google_vision | openai

GOOGLE_VISION_API_KEY=        # required when OCR_PROVIDER=google_vision
OPENAI_API_KEY=               # required when OCR_PROVIDER=openai

# ── Object Storage ────────────────────────────────────────────────────────────
# STORAGE_PROVIDER selects GCS or S3.
STORAGE_PROVIDER=gcs          # options: gcs | s3

STORAGE_BUCKET=gentax-receipts-dev

# GCS: set GOOGLE_APPLICATION_CREDENTIALS to service account JSON path
GOOGLE_APPLICATION_CREDENTIALS=

# S3: set AWS credentials via standard AWS env vars
AWS_REGION=
AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=

# ── Runtime ───────────────────────────────────────────────────────────────────
APP_ENV=development           # options: development | production
LOG_LEVEL=info                # options: debug | info | warn | error

# ── API Server ────────────────────────────────────────────────────────────────
API_PORT=8080
API_CORS_ORIGINS=http://localhost:3000  # comma-separated list for production

# ── Worker ────────────────────────────────────────────────────────────────────
OCR_WORKER_POOL_SIZE=3
OCR_WORKER_POLL_INTERVAL_SECONDS=5
OCR_MAX_RETRIES=3
```

### Configuration Loading Pattern

```go
// internal/config/config.go

type Config struct {
    DatabaseURL     string
    TelegramToken   string
    JWTSecret       string
    OCRProvider     string
    StorageProvider string
    StorageBucket   string
    AppEnv          string
    // ...
}

// Load reads from environment variables. Returns an error listing
// all missing required vars so operators see all problems at once.
func Load() (*Config, error) {
    var missing []string
    get := func(key string) string {
        v := os.Getenv(key)
        if v == "" { missing = append(missing, key) }
        return v
    }

    cfg := &Config{
        DatabaseURL:   get("DATABASE_URL"),
        TelegramToken: get("TELEGRAM_BOT_TOKEN"),
        JWTSecret:     get("JWT_SECRET"),
        OCRProvider:   get("OCR_PROVIDER"),
        // ...
    }

    if len(missing) > 0 {
        return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
    }
    return cfg, nil
}
```

---

## Design Decisions Log

| Decision | Rationale |
|----------|-----------|
| Modular monolith, two `cmd/` binaries | Avoids microservice overhead; Telegram bot and REST API share service layer in-process. Split binaries allows independent deployment if needed. |
| `receipt_id NOT NULL` on `expenses` | Database-level fraud prevention. Every expense requires a physical receipt. No bypass path exists. |
| `FOR UPDATE SKIP LOCKED` for OCR polling | No external queue (Redis, SQS) needed at this scale. PostgreSQL advisory locks provide the same "at most once" semantics. |
| `owner_id` only from JWT, never from request | Eliminates the most dangerous class of multi-tenant data leakage. Auth middleware is the single source of truth. |
| `emit_interface: true` in sqlc | Generates a `Querier` interface automatically, enabling repository mocks without boilerplate. |
| `ocr_raw JSONB` storage | Preserves full OCR provider response for prompt improvements and reprocessing without re-hitting the provider. |
| Conversation FSM in memory (Telegram) | Simplest approach for multi-step expense flow. Acceptable because bot state is ephemeral; a restart only interrupts in-flight conversations, not committed data. If persistence is needed later, Redis can back the FSM. |
| `testcontainers-go` for integration tests | Real PostgreSQL validates sqlc queries, constraint checks, and index behavior that in-memory SQLite mocks would miss. |
