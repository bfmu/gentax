# Tasks: core-expense-tracking

---

## Phase 1: Infrastructure & Project Setup

- [ ] **1.1** Initialize Go module and create full directory skeleton
  - Creates: `go.mod` (`module github.com/bmunoz/gentax`), all `cmd/` and `internal/` directories per design §1, `.gitignore`
  - Dependencies to add: `github.com/go-chi/chi/v5`, `github.com/golang-jwt/jwt/v5`, `github.com/google/uuid`, `github.com/jackc/pgx/v5`, `github.com/pressly/goose/v3` (or `golang-migrate`), `github.com/testcontainers/testcontainers-go`, `github.com/stretchr/testify`, `github.com/stretchr/objx`
  - Tests: `go build ./...` passes with zero errors after stubs are in place

- [ ] **1.2** Write `Makefile` with all required targets
  - Creates: `Makefile` with targets: `build`, `test`, `test-unit`, `test-integration`, `generate`, `migrate-up`, `migrate-down`, `lint`, `run-api`, `run-bot`, `docker-up`, `docker-down` (exact definitions from design §1)
  - Tests: `make build` succeeds; `make test` passes (no code yet, zero tests pass vacuously); `make lint` exits 0

- [ ] **1.3** Create `.env.example` and `internal/config` config loader
  - Creates: `.env.example` with all required vars (`DATABASE_URL`, `JWT_SECRET`, `TELEGRAM_BOT_TOKEN`, `GCS_BUCKET` / `S3_BUCKET`, `LOG_LEVEL`, `SERVER_PORT`)
  - Creates: `internal/config/config.go` — loads vars from environment, returns typed `Config` struct, returns error on missing required vars
  - Tests (unit, `-short`): `TestConfig_LoadsAllRequiredVars`, `TestConfig_ErrorOnMissingJWTSecret`, `TestConfig_ErrorOnMissingDatabaseURL`

- [ ] **1.4** Create `docker-compose.yml` for local PostgreSQL
  - Creates: `docker-compose.yml` with PostgreSQL 16 service, named volume, health-check, env vars matching `.env.example`
  - Tests: `docker compose up -d` starts cleanly; `docker compose down` cleans up (manual smoke test documented in task)

- [ ] **1.5** Set up `sqlc.yaml` configuration
  - Creates: `sqlc.yaml` with `engine: postgresql`, `queries: "query/"`, `schema: "migrations/"`, output to `internal/db/query`, `emit_interface: true`, `emit_json_tags: true`, `null_style: option` (exact config from design §1)
  - Creates: empty placeholder files `query/owners.sql`, `query/taxis.sql`, `query/drivers.sql`, `query/receipts.sql`, `query/expenses.sql` so `sqlc generate` does not error on missing input
  - Tests: `sqlc generate` exits 0 and produces `internal/db/query/db.go` with empty `Querier` interface

---

## Phase 2: Database Schema & Migration Harness

- [ ] **2.1** Write migration 000001: `owners` table
  - Creates: `migrations/000001_create_owners.up.sql`, `migrations/000001_create_owners.down.sql`
  - Schema: `id UUID PK`, `full_name TEXT NOT NULL`, `email TEXT NOT NULL UNIQUE`, `active BOOLEAN DEFAULT TRUE`, `created_at`, `updated_at` (exact DDL from design §3)
  - Tests (integration): `TestMigration_OwnersTableCreated` — runs up migration via testcontainers-go, asserts table exists and constraints hold

- [ ] **2.2** Write migrations 000002–000004: `taxis`, `drivers`, `driver_taxi_assignments`
  - Creates: `migrations/000002_create_taxis.{up,down}.sql`, `000003_create_drivers.{up,down}.sql`, `000004_create_driver_taxi_assignments.{up,down}.sql`
  - Includes all indexes defined in design §3 (e.g. `idx_taxis_owner_id`, `idx_dta_active`)
  - Tests (integration): `TestMigration_TaxisUniquePerOwner` — inserts duplicate plate for same owner, asserts unique constraint fires; `TestMigration_DriversTelegramUnique`; `TestMigration_DTAIndexesExist`

- [ ] **2.3** Write migrations 000005–000007: `expense_categories`, `receipts`, `expenses`
  - Creates: `migrations/000005_create_expense_categories.{up,down}.sql`, `000006_create_receipts.{up,down}.sql`, `000007_create_expenses.{up,down}.sql`
  - `expenses.receipt_id NOT NULL` foreign key, `chk_review_consistency` check constraint, all indexes from design §3
  - Tests (integration): `TestMigration_ExpenseReceiptNotNull` — attempts INSERT with null receipt_id, asserts failure; `TestMigration_CheckReviewConsistency` — attempts approved expense without reviewed_by, asserts failure

- [ ] **2.4** Build `internal/testutil` DB harness
  - Creates: `internal/testutil/db.go` — `NewTestDB(t)` using testcontainers-go (postgres:16-alpine), runs all migrations up, returns `*pgxpool.Pool`, registers `t.Cleanup` teardown (exact code from design §7)
  - Creates: `internal/testutil/fixtures.go` — `CreateOwner`, `CreateDriver`, `CreateTaxi`, `CreateExpenseCategory` seed helpers
  - Creates: `internal/testutil/assert.go` — `AssertExpenseEqual`, `AssertNoOwnerCrossContamination` domain assertions
  - Tests: `TestNewTestDB_StartsAndMigrates` — verifies pool connects and all 7 migrations applied; tagged `//go:build integration`

- [ ] **2.5** Write all `sqlc` SQL query files and run `sqlc generate`
  - Creates: `query/owners.sql` (GetByID, GetByEmail, Create, Update), `query/taxis.sql` (Create, GetByID, ListByOwner, Update), `query/drivers.sql` (Create, GetByID, GetByTelegramID, ListByOwner, LinkTelegram, Update), `query/receipts.sql` (Create, GetByID, ListPendingOCR with `FOR UPDATE SKIP LOCKED`, UpdateOCRResult, MarkOCRFailed, IncrementRetryCount), `query/expenses.sql` (Create, GetByID, List with dynamic filters, Approve, Reject, ListByDriverAndOwner, ReportByTaxi, ReportByDriver, ReportByCategory)
  - Runs: `sqlc generate` — produces all `internal/db/query/*.sql.go` files
  - Tests: `go build ./internal/db/...` passes; `go vet ./internal/db/...` passes

---

## Phase 3: Domain — Auth

- [ ] **3.1** Write failing tests for JWT `TokenIssuer` and `TokenValidator`
  - Creates: `internal/auth/auth_test.go` with table-driven tests:
    - `TestTokenIssuer_Issue_SignsValidJWT`
    - `TestTokenValidator_Validate_AcceptsValidToken`
    - `TestTokenValidator_Validate_RejectsExpiredToken`
    - `TestTokenValidator_Validate_RejectsTamperedSignature`
    - `TestTokenValidator_Validate_RejectsWrongAlgorithm` (alg confusion)
  - All tests must FAIL (red) before implementation — confirms TDD gate
  - REQ: REQ-DRV-03, REQ-FRD-04

- [ ] **3.2** Implement `internal/auth` — `Claims`, `TokenIssuer`, `TokenValidator`
  - Creates: `internal/auth/auth.go` — `Claims` struct, `JWTIssuer` (implements `TokenIssuer`), `JWTValidator` (implements `TokenValidator`) using `golang-jwt/jwt/v5`, HS256 signing, 1-hour expiry
  - All tests from 3.1 must PASS (green)
  - Tests: run `go test -race -cover ./internal/auth/...`; target ≥ 90% coverage

- [ ] **3.3** Implement `internal/auth/middleware.go` — Chi JWT middleware
  - Creates: `internal/auth/middleware.go` — `RequireAuth(validator TokenValidator)` Chi middleware that extracts `Authorization: Bearer <token>`, validates it, injects `Claims` into `context.Context` via `auth.ClaimsKey`
  - Creates: `internal/auth/middleware_test.go` with:
    - `TestRequireAuth_PassesValidToken`
    - `TestRequireAuth_Returns401OnMissingToken`
    - `TestRequireAuth_Returns401OnExpiredToken`
    - `TestRequireAuth_Returns403OnWrongRole`
  - REQ: cross-cutting auth requirements, REQ-FRD-04

---

## Phase 4: Domain — Taxi

- [ ] **4.1** Write failing tests for `taxi.Service`
  - Creates: `internal/taxi/taxi_test.go` with table-driven unit tests:
    - `TestTaxiService_Register_CreatesWithOwnerID`
    - `TestTaxiService_Register_RejectsDuplicatePlate` (expects domain error mapped to 409)
    - `TestTaxiService_Register_RejectsYearBelow1990` (expects 422)
    - `TestTaxiService_Register_RejectsYearAboveCurrentPlusOne`
    - `TestTaxiService_Deactivate_SetsActiveFalse`
    - `TestTaxiService_Get_Returns404ForWrongOwner`
    - `TestTaxiService_List_OnlyReturnsOwnerTaxis`
  - All tests FAIL before implementation
  - REQ: REQ-TAX-01 through REQ-TAX-05

- [ ] **4.2** Implement `internal/taxi` — entity, interfaces, service, mock
  - Creates: `internal/taxi/taxi.go` — `Taxi` struct, `Repository` interface, `Service` interface, `service` implementation with year validation and owner-scoped operations (exact interfaces from design §2)
  - Creates: `internal/taxi/mock_repository.go` — testify/mock stub implementing `Repository`
  - All tests from 4.1 PASS
  - Tests: `go test -race -cover ./internal/taxi/...`; target ≥ 80% coverage

- [ ] **4.3** Implement `taxi.Repository` backed by sqlc
  - Creates: `internal/taxi/repository.go` — `sqlcRepository` implementing `taxi.Repository`, delegates to `internal/db/query`, every method includes `owner_id` param (no unscoped queries)
  - Creates: `internal/taxi/repository_integration_test.go` (tagged `//go:build integration`):
    - `TestTaxiRepository_CreateAndGet`
    - `TestTaxiRepository_UniqueConstraintPerOwner`
    - `TestTaxiRepository_ListByOwnerIsolation` — two owners, assert zero cross-contamination
  - REQ: REQ-TNT-01

---

## Phase 5: Domain — Driver

- [ ] **5.1** Write failing tests for `driver.Service`
  - Creates: `internal/driver/driver_test.go` with table-driven tests:
    - `TestDriverService_Register_CreatesWithOwnerID`
    - `TestDriverService_Register_RejectsBlankFullName` (expects 422)
    - `TestDriverService_LinkTelegramID_SingleUseToken`
    - `TestDriverService_LinkTelegramID_RejectsExpiredToken`
    - `TestDriverService_LinkTelegramID_RejectsDuplicateTelegramIDSameOwner`
    - `TestDriverService_Deactivate_UnassignsTaxi`
    - `TestDriverService_Deactivate_PreventsNewExpenses`
    - `TestDriverService_List_OnlyReturnsOwnerDrivers`
  - All tests FAIL before implementation
  - REQ: REQ-DRV-01 through REQ-DRV-05

- [ ] **5.2** Implement `internal/driver` — entity, interfaces, service, mock
  - Creates: `internal/driver/driver.go` — `Driver` struct, `Repository` interface, `Service` interface, `service` implementation; link token generation (24h expiry, single-use via in-memory or DB token table)
  - Creates: `internal/driver/mock_repository.go`
  - All tests from 5.1 PASS
  - Tests: `go test -race -cover ./internal/driver/...`; target ≥ 80% coverage

- [ ] **5.3** Implement `driver.Repository` backed by sqlc + integration tests
  - Creates: `internal/driver/repository.go` — `sqlcRepository` wrapping `internal/db/query`; every method owner-scoped
  - Creates: `internal/driver/repository_integration_test.go` (tagged `//go:build integration`):
    - `TestDriverRepository_LinkTelegram_UniquePerOwner`
    - `TestDriverRepository_AssignTaxi_RejectsDualAssignment` (REQ-TAX-03)
    - `TestDriverRepository_UnassignTaxi_SetsUnassignedAt` (REQ-TAX-04)
    - `TestDriverRepository_ListByOwnerIsolation`
  - REQ: REQ-DRV-02, REQ-TAX-03, REQ-TAX-04, REQ-TNT-01

---

## Phase 6: Domain — Receipt & OCR

- [ ] **6.1** Write failing tests for `receipt.Repository` and DIAN NIT validator
  - Creates: `internal/receipt/nit_test.go` with exhaustive table-driven tests:
    - `TestValidateNIT_AcceptsValidNIT` (known-good DIAN NITs)
    - `TestValidateNIT_RejectsWrongCheckDigit`
    - `TestValidateNIT_RejectsNonNumeric`
    - `TestValidateNIT_AcceptsWithAndWithoutHyphen`
    - `TestValidateNIT_RejectsTooShortOrTooLong`
  - Creates: `internal/receipt/receipt_test.go` — unit tests with mock `OCRClient` and mock `Repository`:
    - `TestReceiptProcessor_Process_UploadsBeforeDBInsert` (REQ-FRD-02)
    - `TestReceiptProcessor_Process_AbortsOnStorageFailure` (REQ-FRD-02)
    - `TestReceiptProcessor_Process_SetsStorageURLNotNull` (REQ-FRD-02)
  - All tests FAIL before implementation
  - REQ: REQ-OCR-02, REQ-FRD-02

- [ ] **6.2** Implement `internal/receipt` — entity, interfaces, NIT validator, processor, mocks
  - Creates: `internal/receipt/receipt.go` — `Receipt` struct, `OCRStatus` constants, `Repository` interface, `OCRClient` interface, `Processor` interface, `StorageClient` interface (for GCS/S3 swappability), `ExtractedData` struct (exact definitions from design §2)
  - Creates: `internal/receipt/nit.go` — `ValidateNIT(raw string) error` using DIAN weights `[3,7,13,17,19,23,29,37,41,43]` (exact algorithm from design §6)
  - Creates: `internal/receipt/processor.go` — `Processor` implementation: download from Telegram → upload to storage → `Repository.Create` (aborts entire flow on storage failure)
  - Creates: `internal/receipt/mock_ocr_client.go`, `internal/receipt/mock_repository.go`, `internal/receipt/mock_storage_client.go`
  - All tests from 6.1 PASS
  - Tests: `go test -race -cover ./internal/receipt/...`; target ≥ 80% coverage

- [ ] **6.3** Implement `receipt.Repository` backed by sqlc + integration tests
  - Creates: `internal/receipt/repository.go` — `sqlcRepository`; `ListPendingOCR` uses `FOR UPDATE SKIP LOCKED`
  - Creates: `internal/receipt/repository_integration_test.go` (tagged `//go:build integration`):
    - `TestReceiptRepository_StorageURLNotNull` — asserts DB rejects null `storage_url`
    - `TestReceiptRepository_ListPendingOCR_SkipsLocked` — verifies concurrent callers do not double-process
    - `TestReceiptRepository_UpdateOCRResult_SetsStatusDone`
    - `TestReceiptRepository_MarkOCRFailed_SetsStatusFailed`
  - REQ: REQ-OCR-01, REQ-FRD-02

- [ ] **6.4** Implement `internal/worker/ocr_worker.go` — goroutine pool + tests
  - Creates: `internal/worker/ocr_worker.go` — `OCRWorker` struct with `Run(ctx)`, `processBatch`, `processOne`, `TelegramNotifier` interface; pool size 3, poll interval 5s, max retries 3 (constants from design §6); NIT validation on extracted data; updates linked `expenses.amount_cop` when `extracted_total` present and expense amount is null
  - Creates: `internal/worker/ocr_worker_test.go` with mock `OCRClient`, mock `Repository`, mock `TelegramNotifier`:
    - `TestOCRWorker_ProcessOne_SuccessUpdatesReceipt`
    - `TestOCRWorker_ProcessOne_SuccessUpdatesExpenseAmount`
    - `TestOCRWorker_ProcessOne_FailureIncrementsRetryCount`
    - `TestOCRWorker_ProcessOne_MaxRetriesMarksFailed`
    - `TestOCRWorker_ProcessOne_InvalidNITStoredEmpty`
    - `TestOCRWorker_ProcessOne_NotifiesDriverOnSuccess` (REQ-OCR-04)
    - `TestOCRWorker_ProcessOne_NotifiesDriverOnFailure` (REQ-OCR-04)
    - `TestOCRWorker_ProcessOne_ContinuesOnTelegramSendFailure` (REQ-OCR-04)
  - Tests: `go test -race -cover ./internal/worker/...`; target ≥ 80% coverage
  - REQ: REQ-OCR-01 through REQ-OCR-04

---

## Phase 7: Domain — Expense

- [ ] **7.1** Write failing tests for `expense.Service` — state machine and multi-tenant isolation
  - Creates: `internal/expense/expense_test.go` with table-driven tests:
    - `TestExpenseService_Create_RequiresReceiptID` (REQ-FRD-01)
    - `TestExpenseService_Create_SetsPendingStatus`
    - `TestExpenseService_Create_ValidatesOwnerMatchesDriverOwner`
    - `TestExpenseService_Create_ValidatesOwnerMatchesTaxiOwner`
    - `TestExpenseService_Approve_RequiresConfirmedStatus` (REQ-FRD-03, REQ-APR-02)
    - `TestExpenseService_Approve_RejectsPendingExpense`
    - `TestExpenseService_Approve_RejectsAlreadyApproved`
    - `TestExpenseService_Reject_RequiresConfirmedStatus` (REQ-APR-03)
    - `TestExpenseService_Get_Returns404ForWrongOwner` (REQ-TNT-03)
    - State-machine table test covering all invalid transitions (pending→approved, approved→rejected, rejected→approved, etc.)
  - All tests FAIL before implementation
  - REQ: REQ-APR-01 through REQ-APR-04, REQ-FRD-01, REQ-FRD-03, REQ-TNT-01 through REQ-TNT-03

- [ ] **7.2** Implement `internal/expense` — entity, interfaces, service, mock
  - Creates: `internal/expense/expense.go` — `Expense` struct, `Status` constants (`pending`, `confirmed`, `approved`, `rejected` — note: `confirmed` is a valid intermediate status per spec; add to design's Status const list), `ListFilter` struct, `Repository` interface, `Service` interface, `service` implementation enforcing state machine and owner scoping (exact interfaces from design §2)
  - Creates: `internal/expense/mock_repository.go`
  - All tests from 7.1 PASS
  - Tests: `go test -race -cover ./internal/expense/...`; target ≥ 85% coverage

- [ ] **7.3** Implement `expense.Repository` backed by sqlc + integration tests
  - Creates: `internal/expense/repository.go` — `sqlcRepository`; `List` supports all `ListFilter` fields; `Approve`/`Reject` use conditional UPDATE `WHERE status = 'confirmed'`
  - Creates: `internal/expense/repository_integration_test.go` (tagged `//go:build integration`):
    - `TestExpenseRepository_CreateWithReceiptID`
    - `TestExpenseRepository_ApproveOnlyConfirmed` — assert DB constraint fires for wrong status
    - `TestExpenseRepository_RejectSendsRejectionReason`
    - `TestExpenseRepository_ListFilters_ByTaxi`, `_ByDriver`, `_ByDateRange`, `_ByCategory`, `_ByStatus`
    - `TestExpenseRepository_MultiTenantIsolation` — two owners, seed data for each, assert zero cross-contamination on every query path (REQ-TNT-01)
  - REQ: REQ-APR-01 through REQ-APR-04, REQ-RPT-01, REQ-TNT-01

- [ ] **7.4** Write `internal/testutil` multi-tenant isolation integration test
  - Creates: `internal/testutil/isolation_test.go` (tagged `//go:build integration`) — `TestMultiTenantIsolation` seeds Owner A and Owner B with taxis, drivers, expenses; calls every List/Get method with each owner's ID; asserts zero cross-contamination on all 6 tables (exact test structure from design §7)
  - REQ: REQ-TNT-01, REQ-TNT-02, REQ-TNT-03

---

## Phase 8: REST API

- [ ] **8.1** Write failing handler tests for auth and taxi endpoints
  - Creates: `internal/httpapi/handlers/auth_test.go`:
    - `TestAuthHandler_TelegramAuth_IssuesJWT`
    - `TestAuthHandler_TelegramAuth_Returns401OnUnknownTelegramID`
  - Creates: `internal/httpapi/handlers/taxis_test.go` using `httptest.NewRecorder`:
    - `TestTaxiHandler_Create_Returns201`
    - `TestTaxiHandler_Create_Returns409OnDuplicatePlate`
    - `TestTaxiHandler_Create_Returns422OnInvalidYear`
    - `TestTaxiHandler_Create_IgnoresOwnerIDInBody` (REQ-FRD-04)
    - `TestTaxiHandler_List_Returns200WithOwnerTaxisOnly`
    - `TestTaxiHandler_Get_Returns404ForWrongOwner`
    - `TestTaxiHandler_Deactivate_Returns200`
    - `TestTaxiHandler_AssignDriver_Returns409OnDualAssignment`
    - `TestTaxiHandler_UnassignDriver_Returns404WhenNoActiveAssignment`
  - All tests FAIL before implementation
  - REQ: REQ-TAX-01 through REQ-TAX-05

- [ ] **8.2** Implement chi router, middleware stack, and auth + taxi handlers
  - Creates: `internal/httpapi/router.go` — Chi router, middleware chain: logging (`slog`), CORS, `RequireAuth`, role enforcement helper; registers all routes from design §5 endpoint table
  - Creates: `internal/httpapi/middleware/auth.go`, `logging.go`, `cors.go`
  - Creates: `internal/httpapi/handlers/auth.go` — `POST /auth/telegram`
  - Creates: `internal/httpapi/handlers/taxis.go` — all taxi endpoints; owner_id always from context claims
  - All tests from 8.1 PASS
  - Tests: `go test -race -cover ./internal/httpapi/...`; target ≥ 75% coverage

- [ ] **8.3** Write failing handler tests for driver and expense endpoints
  - Creates: `internal/httpapi/handlers/drivers_test.go`:
    - `TestDriverHandler_Create_Returns201`
    - `TestDriverHandler_Create_Returns422OnBlankName`
    - `TestDriverHandler_List_IncludesActiveTaxiAssignment`
    - `TestDriverHandler_Deactivate_Returns200`
  - Creates: `internal/httpapi/handlers/expenses_test.go`:
    - `TestExpenseHandler_List_SupportsCombinedFilters` (REQ-RPT-01)
    - `TestExpenseHandler_List_PaginatedDefaultSize20`
    - `TestExpenseHandler_Get_Returns404ForOtherOwner` (REQ-TNT-03)
    - `TestExpenseHandler_Approve_Returns409OnNonConfirmedStatus` (REQ-APR-02)
    - `TestExpenseHandler_Reject_StoresRejectionReason` (REQ-APR-03)
    - `TestExpenseHandler_Approve_Returns403ForDriverRole`
  - All tests FAIL before implementation
  - REQ: REQ-DRV-01 through REQ-DRV-05, REQ-APR-01 through REQ-APR-04

- [ ] **8.4** Implement driver and expense handlers
  - Creates: `internal/httpapi/handlers/drivers.go` — all driver endpoints
  - Creates: `internal/httpapi/handlers/expenses.go` — all expense endpoints; reject notification to driver on reject (REQ-APR-03)
  - All tests from 8.3 PASS

- [ ] **8.5** Write failing tests and implement report handlers
  - Creates: `internal/httpapi/handlers/reports_test.go`:
    - `TestReportHandler_ExpenseList_FiltersAndPaginates` (REQ-RPT-01)
    - `TestReportHandler_TaxiSummary_OnlyApprovedExpenses` (REQ-RPT-02)
    - `TestReportHandler_TaxiSummary_IncludesTaxisWithZeroExpenses` (REQ-RPT-02)
    - `TestReportHandler_DriverSummary_IncludesInactiveDrivers` (REQ-RPT-03)
    - `TestReportHandler_CategoryBreakdown_ScopeToOwner` (REQ-RPT-04)
  - Creates: `internal/httpapi/handlers/reports.go` — `GET /reports/expenses`, `GET /reports/taxis`, `GET /reports/drivers`, `GET /reports/categories`
  - All tests PASS
  - REQ: REQ-RPT-01 through REQ-RPT-04

- [ ] **8.6** Write error response middleware — uniform JSON error format
  - Creates: `internal/httpapi/middleware/errors.go` — maps domain error types to HTTP status codes; all error responses return `{"error": "<message>", "code": "<machine-code>"}` (cross-cutting requirement from spec §error-handling)
  - Creates: `internal/httpapi/middleware/errors_test.go`:
    - `TestErrorMiddleware_MapsNotFoundTo404`
    - `TestErrorMiddleware_MapsConflictTo409`
    - `TestErrorMiddleware_MapsValidationTo422`
    - `TestErrorMiddleware_HidesinternalDetailsOn500`

---

## Phase 9: Telegram Bot

- [ ] **9.1** Write failing tests for `/start` command handler
  - Creates: `internal/telegram/bot_test.go` with mock `driver.Service` and mock `auth.TokenIssuer`:
    - `TestBot_HandleStart_IssuesJWTForLinkedDriver` (REQ-DRV-03)
    - `TestBot_HandleStart_PromptsLinkFlowForUnknownTelegramID` (REQ-DRV-02)
    - `TestBot_HandleStart_RejectsInactiveDriver` (REQ-DRV-04)
  - All tests FAIL before implementation

- [ ] **9.2** Implement `internal/telegram` bot core and `/start` handler
  - Creates: `internal/telegram/bot.go` — `Bot` struct wrapping `go-telegram-bot-api` (or `telebot`), update dispatcher, long-poll loop
  - Creates: `internal/telegram/conversation.go` — in-memory FSM per `telegram_user_id` tracking expense flow state (states: idle → taxi-select → category-select → awaiting-photo | awaiting-amount → done)
  - Creates: `internal/telegram/handlers.go` — `handleStart`: looks up driver by Telegram ID, issues JWT, stores claims in FSM state
  - All tests from 9.1 PASS

- [ ] **9.3** Write failing tests for `/expense` multi-step flow
  - Creates additional tests in `internal/telegram/bot_test.go`:
    - `TestBot_HandleExpense_PromptsSelectTaxi` (REQ-EXP-01)
    - `TestBot_HandleExpense_NoActiveTaxiBlocksFlow` (REQ-EXP-01)
    - `TestBot_HandleExpense_SelectTaxiPromptsCategoryList` (REQ-EXP-02)
    - `TestBot_HandlePhoto_UploadsAndCreatesReceiptAndExpense` (REQ-EXP-03)
    - `TestBot_HandlePhoto_AbortsOnStorageFailure` (REQ-EXP-03)
    - `TestBot_HandleManualAmount_CreatesPendingExpense` (REQ-EXP-04)
    - `TestBot_HandleOCRConfirm_SetsExpenseConfirmed` (REQ-OCR-05)
    - `TestBot_HandleOCREdit_AcceptsCorrectedAmountAndConfirms` (REQ-OCR-05)
    - `TestBot_HandleOCRConfirm_RejectsOtherDriversExpense` (REQ-TNT-02)
  - All tests FAIL before implementation
  - REQ: REQ-EXP-01 through REQ-EXP-04, REQ-OCR-05

- [ ] **9.4** Implement `/expense` flow and OCR callback handlers
  - Extends: `internal/telegram/handlers.go` — `handleExpenseCommand`, `handleTaxiSelection` (callback), `handleCategorySelection` (callback), `handlePhoto`, `handleManualAmount`, `handleOCRConfirm`, `handleOCREdit`
  - Extends: `internal/telegram/conversation.go` — FSM transitions for full expense flow
  - All tests from 9.3 PASS

- [ ] **9.5** Write failing tests and implement `/status` command
  - Creates tests:
    - `TestBot_HandleStatus_ShowsLast10Expenses` (REQ-EXP-05)
    - `TestBot_HandleStatus_ScopedToDriverAndOwner` (REQ-TNT-02, REQ-EXP-05)
    - `TestBot_HandleStatus_UnauthenticatedPromptsStartFlow`
  - Implements `handleStatus` in `internal/telegram/handlers.go`
  - All tests PASS
  - REQ: REQ-EXP-05, REQ-TNT-02

---

## Phase 10: cmd Binaries & End-to-End Wiring

- [ ] **10.1** Implement `cmd/api/main.go` — wire all API dependencies
  - Creates: `cmd/api/main.go` — loads `internal/config`, creates pgxpool connection (`internal/db/conn.go`), runs migrations (`internal/db/migrate.go`), constructs all repositories, services, handlers, chi router; starts HTTP server; handles graceful shutdown on SIGINT/SIGTERM
  - Creates: `internal/db/conn.go` — `NewPool(ctx, databaseURL) (*pgxpool.Pool, error)`
  - Creates: `internal/db/migrate.go` — `RunMigrationsUp(connStr string) error`, `RunMigrationsDown(connStr string) error` using golang-migrate
  - Tests: `make build` succeeds; `go vet ./cmd/api/...` passes

- [ ] **10.2** Implement `cmd/bot/main.go` — wire all bot dependencies
  - Creates: `cmd/bot/main.go` — loads config, creates pgxpool, constructs repositories, services, `receipt.Processor` (with real `StorageClient`), `OCRWorker` (starts in separate goroutine), Telegram `Bot`; starts long-poll loop; graceful shutdown
  - Tests: `make build` succeeds; `go vet ./cmd/bot/...` passes

- [ ] **10.3** End-to-end smoke test (integration)
  - Creates: `internal/testutil/e2e_test.go` (tagged `//go:build integration`):
    - `TestE2E_BotSubmitsExpenseOCRWorkerProcessesAndConfirms`:
      1. Spin up testcontainers PostgreSQL, apply migrations
      2. Create owner, driver, taxi, expense category via repositories
      3. Link fake telegram_id to driver
      4. Simulate bot `/expense` flow with a fake photo (stub storage + stub OCR returning valid DIAN fields)
      5. Assert `receipts.ocr_status = 'done'`
      6. Assert `expenses.amount_cop` updated from OCR extracted_total
      7. Assert driver received Telegram notification (mock notifier call verified)
  - REQ: REQ-EXP-03, REQ-OCR-01 through REQ-OCR-04

- [ ] **10.4** CI configuration and coverage gate
  - Creates: `.github/workflows/ci.yml` (or equivalent) — runs `make lint`, `make test` (unit), `make test-integration`; fails if any package drops below coverage targets from design §7
  - Adds `//go:build integration` build tag consistently across all integration test files
  - Verifies: `make test` (unit only, `-short`) passes in under 30 seconds with no external deps

---

## Dependency Order Summary

```
1.1 → 1.2 → 1.3 → 1.4 → 1.5
                              ↓
2.1 → 2.2 → 2.3 → 2.4 → 2.5
                              ↓
              3.1 → 3.2 → 3.3
              4.1 → 4.2 → 4.3
              5.1 → 5.2 → 5.3
              6.1 → 6.2 → 6.3 → 6.4
              7.1 → 7.2 → 7.3 → 7.4
                              ↓
              8.1 → 8.2 → 8.3 → 8.4 → 8.5 → 8.6
              9.1 → 9.2 → 9.3 → 9.4 → 9.5
                              ↓
              10.1 → 10.2 → 10.3 → 10.4
```

Phases 3–7 can proceed in parallel after Phase 2 completes.
Phases 8 and 9 can proceed in parallel after their respective domain phases complete.
Phase 10 requires both Phase 8 and Phase 9 to be complete.
