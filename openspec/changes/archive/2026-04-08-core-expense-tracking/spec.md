# Specs: core-expense-tracking

## Overview

These specifications govern the core expense tracking system for **gentax**, a taxi fleet expense management platform operating in Bogotá, Colombia. All monetary amounts are in COP. Authentication is Telegram ID → JWT (1h). Every data-access requirement is owner-scoped; multi-tenancy isolation is a cross-cutting invariant enforced at the service and database layers.

RFC 2119 keywords (MUST, SHALL, SHOULD, MAY) are used throughout.

---

## 1. Taxi Management

### REQ-TAX-01: Create Taxi

**Given** an authenticated owner with a valid JWT (role=admin)  
**When** the owner submits a create-taxi request with `plate` (string, 6-char Colombian format), `model` (string), and `year` (integer, 1990–current year+1)  
**Then** the system MUST create a `taxis` record with `owner_id` set from the JWT claims, `active=true`, and return the created taxi with its generated `id`

- The `plate` field MUST be unique per owner. A duplicate plate for the same owner MUST return HTTP 409 Conflict.
- The `year` field MUST be validated: values below 1990 or above the current calendar year + 1 MUST return HTTP 422 Unprocessable Entity.
- `owner_id` MUST be taken exclusively from JWT claims. Clients MUST NOT supply `owner_id` as a request field.

---

### REQ-TAX-02: List Taxis

**Given** an authenticated owner  
**When** the owner requests the taxi list  
**Then** the system MUST return only taxis where `owner_id` matches the JWT claim, ordered by `created_at` descending

- The response MUST include `active` status for each taxi.
- Inactive taxis SHOULD be included in list responses unless the caller specifies `active=true` query parameter.

---

### REQ-TAX-03: Assign Driver to Taxi

**Given** an authenticated owner  
**And** both the driver and taxi belong to that owner  
**When** the owner creates a driver-taxi assignment  
**Then** the system MUST insert a `driver_taxi_assignments` record with `assigned_at=now()` and no `unassigned_at`

- A driver MUST NOT be assigned to two taxis simultaneously under the same owner. Attempting this MUST return HTTP 409 Conflict.
- The system MUST verify that both the `driver_id` and `taxi_id` belong to the requesting owner before creating the assignment. A mismatch MUST return HTTP 404 Not Found (not 403, to avoid enumeration).

---

### REQ-TAX-04: Unassign Driver from Taxi

**Given** an authenticated owner  
**And** an active assignment between a driver and a taxi belonging to that owner  
**When** the owner requests unassignment  
**Then** the system MUST set `unassigned_at=now()` on the assignment record rather than deleting it

- Historical assignments MUST be preserved for audit and reporting purposes.
- If no active assignment exists for the given pair, the system MUST return HTTP 404 Not Found.

---

### REQ-TAX-05: Deactivate Taxi

**Given** an authenticated owner  
**And** an existing active taxi belonging to that owner  
**When** the owner requests deactivation  
**Then** the system MUST set `active=false` on the taxi record

- Deactivating a taxi MUST NOT delete its historical expense or assignment records.
- Drivers MUST NOT be able to register new expenses against an inactive taxi. Attempts MUST be rejected with a clear error message.
- Deactivation is soft-delete only; hard deletes of taxis are out of scope.

---

## 2. Driver Management

### REQ-DRV-01: Create Driver

**Given** an authenticated owner  
**When** the owner submits a create-driver request with `full_name` (string, non-empty), `phone` (string), and optionally `telegram_id` (integer)  
**Then** the system MUST create a `drivers` record with `owner_id` from JWT claims and `active=true`

- `telegram_id` MAY be omitted at creation and linked later via the Telegram bot `/start` flow.
- `phone` MUST be stored as-is; format validation is RECOMMENDED but not required in v1.
- `full_name` MUST NOT be blank. An empty or whitespace-only name MUST return HTTP 422.

---

### REQ-DRV-02: Link Telegram ID to Driver

**Given** a driver record exists with no `telegram_id` set  
**And** the driver sends `/start` to the Telegram bot with a valid link token issued by the owner  
**When** the bot processes the `/start` command  
**Then** the system MUST set `telegram_id` on the matching `drivers` record

- The link token MUST be single-use and expire after 24 hours.
- If the Telegram ID is already linked to another driver under the same owner, the system MUST reject the link and notify the driver via Telegram.
- `telegram_id` MUST be unique across all drivers belonging to the same owner.

---

### REQ-DRV-03: Authenticate Driver via Telegram Bot

**Given** a driver with a linked `telegram_id`  
**When** the driver initiates an authenticated action via the Telegram bot  
**Then** the system MUST issue a JWT with claims `{ user_id, role: "driver", owner_id, driver_id }` valid for 1 hour

- The JWT MUST be signed with the application's secret loaded from environment variables (never hardcoded).
- Unauthenticated bot interactions (no linked `telegram_id`) MUST prompt the driver to complete the `/start` linking flow.

---

### REQ-DRV-04: Deactivate Driver

**Given** an authenticated owner  
**And** an existing active driver belonging to that owner  
**When** the owner requests driver deactivation  
**Then** the system MUST set `active=false` on the driver record

- Deactivation MUST NOT delete historical expense or assignment records.
- An inactive driver MUST NOT be able to register new expenses via the Telegram bot. Bot interactions from an inactive driver MUST return a clear rejection message.
- Any active taxi assignment for the driver SHOULD be automatically unassigned (`unassigned_at=now()`) on deactivation.

---

### REQ-DRV-05: List Drivers

**Given** an authenticated owner  
**When** the owner requests the driver list  
**Then** the system MUST return only drivers where `owner_id` matches the JWT claim

- The response MUST include `active` status and current taxi assignment (if any).

---

## 3. Expense Registration (via Telegram Bot)

### REQ-EXP-01: Initiate Expense Registration Flow

**Given** an authenticated driver with an active account and at least one active taxi assigned to them  
**When** the driver sends `/expense` to the Telegram bot  
**Then** the bot MUST begin a multi-step conversation flow and prompt the driver to select a taxi from their currently assigned taxis

- If the driver has no active taxi assignment, the bot MUST reply with a clear message explaining that no taxi is currently assigned and expense submission is not possible.
- The conversation state MUST be tracked in-memory (or in a fast ephemeral store) per `telegram_id` for the duration of the flow.

---

### REQ-EXP-02: Select Taxi and Category

**Given** the driver is in the expense registration flow  
**When** the driver selects a taxi (via inline keyboard button)  
**Then** the bot MUST present the list of active `expense_categories` for that owner as inline keyboard buttons

**When** the driver selects a category  
**Then** the bot MUST prompt the driver to send a receipt photo or enter the amount manually

- Expense categories MUST be fetched from the `expense_categories` table filtered by `owner_id`.
- The driver MUST NOT be able to proceed past taxi selection without selecting a valid taxi from the presented list.

---

### REQ-EXP-03: Submit Receipt Photo

**Given** the driver is in the expense registration flow with taxi and category selected  
**When** the driver sends a photo message to the bot  
**Then** the system MUST:
  1. Download the photo from Telegram and store it in persistent object storage (GCS or S3-compatible) immediately
  2. Create a `receipts` record with `ocr_status=pending`, `storage_url` pointing to the stored file, and `telegram_file_id` as a secondary reference only
  3. Create an `expenses` record with `status=pending`, `receipt_id` linked to the new receipt, `driver_id`, `taxi_id`, `category_id`, and `owner_id` all set correctly
  4. Confirm to the driver via Telegram that the expense has been submitted and is pending OCR processing

- The `storage_url` MUST be set before the receipt record is committed. A receipt MUST NOT be created with a null `storage_url`.
- The `telegram_file_id` MUST be stored but MUST NOT be used as the authoritative source for the photo.
- If the object storage upload fails, the bot MUST notify the driver and abort the flow without creating any database records.

---

### REQ-EXP-04: Manual Amount Entry (OCR Fallback)

**Given** the driver is in the expense registration flow  
**When** the driver chooses to enter the amount manually (or OCR fails and the system prompts manual entry)  
**Then** the bot MUST accept a numeric text input representing the amount in COP (whole pesos or with up to 2 decimal places)

- The entered amount MUST be stored in `expenses.amount`.
- Even for manual entry, a receipt record MUST be created (with `ocr_status=skipped` if no photo was provided, or `ocr_status=failed` if OCR failed).
- An expense MUST NOT exist without a linked receipt record.

---

### REQ-EXP-05: Expense Status Check

**Given** an authenticated driver  
**When** the driver sends `/status` to the bot  
**Then** the bot MUST display the driver's last 10 expenses with their current status (`pending`, `approved`, `rejected`)

- Results MUST be scoped to the driver's `owner_id` and `driver_id`. The driver MUST NOT see expenses from other drivers or owners.

---

## 4. OCR Pipeline

### REQ-OCR-01: Worker Picks Up Pending Receipts

**Given** one or more `receipts` records exist with `ocr_status=pending`  
**When** the background worker polls the database  
**Then** the worker MUST select pending receipts using `SELECT ... FOR UPDATE SKIP LOCKED` to prevent duplicate processing in concurrent environments

- The worker MUST poll at a configurable interval (default: 10 seconds).
- The worker MUST process receipts in `created_at` ascending order (oldest first).
- The worker MUST handle database errors gracefully: a failed poll MUST log the error and retry on the next interval without crashing.

---

### REQ-OCR-02: Extract DIAN Fields from Receipt

**Given** the worker has selected a pending receipt  
**When** the worker calls `receipt.OCRClient.ExtractData(storageURL)` with the receipt's `storage_url`  
**Then** the OCR client MUST attempt to extract the following DIAN electronic invoice fields:
  - `extracted_nit` — vendor's NIT (digits + check digit, DIAN format)
  - `extracted_cufe` — Código Único de Factura Electrónica
  - `extracted_total` — total amount in COP (`NUMERIC(12,2)`)
  - `extracted_date` — invoice date
  - `extracted_concept` — vendor name or expense concept

- The raw OCR provider response MUST be stored in `receipts.ocr_raw` (JSONB) regardless of success or failure.
- The `OCRClient` interface MUST be injected as a dependency. The concrete provider (Google Cloud Vision or GPT-4o Vision) MUST be swappable without changing the worker logic.

---

### REQ-OCR-03: Update Receipt After OCR

**Given** OCR extraction completes successfully  
**When** the worker updates the receipt record  
**Then** the system MUST set `ocr_status=done`, populate all extracted DIAN fields, and update `expenses.amount` with `extracted_total` if `expenses.amount` is currently null

**Given** OCR extraction fails (provider error, unreadable image, missing fields)  
**When** the worker handles the failure  
**Then** the system MUST set `ocr_status=failed`, store the error details in `ocr_raw`, and leave `expenses.amount` unchanged

- NIT format validation (digits + check digit per DIAN rules) SHOULD be applied to `extracted_nit` before storing. Invalid NIT values SHOULD be stored as-is with a validation flag rather than discarded.
- A receipt with `ocr_status=failed` MUST NOT block the expense from proceeding; manual correction by the driver is the fallback path.

---

### REQ-OCR-04: Notify Driver After OCR

**Given** the worker has updated the receipt (success or failure)  
**When** the worker sends a Telegram notification to the driver  
**Then**:
- On success: the bot MUST display the extracted fields (NIT, total, date, concept) and ask the driver to confirm or correct the data via inline keyboard buttons (Confirm / Edit)
- On failure: the bot MUST notify the driver that automatic extraction failed and prompt for manual amount entry

- The notification MUST be sent to the `telegram_id` of the driver linked to the expense.
- If the driver's `telegram_id` is null or the Telegram send fails, the worker MUST log the error and continue without crashing. The expense remains in `status=pending` for the admin to handle.

---

### REQ-OCR-05: Driver Confirms or Corrects OCR Data

**Given** the driver received an OCR confirmation notification  
**When** the driver taps "Confirm"  
**Then** the system MUST set `expenses.status=confirmed` (awaiting admin approval) and acknowledge the driver

**When** the driver taps "Edit"  
**Then** the bot MUST allow the driver to enter a corrected amount in COP  
**And** upon submission MUST update `expenses.amount` with the corrected value and set `expenses.status=confirmed`

- A driver MUST only be able to confirm or edit their own expenses.
- Confirmation MUST NOT change `ocr_status` on the receipt; OCR data is immutable once written.

---

## 5. Expense Approval (Admin)

### REQ-APR-01: Owner Views Pending Expenses

**Given** an authenticated owner  
**When** the owner requests the pending expenses list  
**Then** the system MUST return all expenses where `owner_id` matches the JWT claim and `status=confirmed`

- The response MUST include: `expense_id`, `driver` (full name), `taxi` (plate), `category`, `amount`, `receipt_storage_url`, `created_at`, and extracted DIAN fields from the linked receipt.
- Results MUST be paginated (default page size: 20).

---

### REQ-APR-02: Owner Approves Expense

**Given** an authenticated owner  
**And** an expense in `status=confirmed` belonging to that owner  
**When** the owner submits an approval  
**Then** the system MUST set `expenses.status=approved` and record `approved_at=now()` and `approved_by=owner_id`

- The expense MUST have a linked receipt with `receipt_id NOT NULL` before approval is permitted. The service layer MUST enforce this check; the database constraint is an additional safety net.
- Only expenses in `status=confirmed` MAY be approved. Attempting to approve an expense in `status=pending`, `approved`, or `rejected` MUST return HTTP 409 Conflict.

---

### REQ-APR-03: Owner Rejects Expense

**Given** an authenticated owner  
**And** an expense in `status=confirmed` belonging to that owner  
**When** the owner submits a rejection with an optional `rejection_reason` (string)  
**Then** the system MUST set `expenses.status=rejected`, record `rejected_at=now()`, and store `rejection_reason`

- Only expenses in `status=confirmed` MAY be rejected. Other statuses MUST return HTTP 409 Conflict.
- After rejection, the system MUST send a Telegram notification to the driver with the rejection reason (if provided).
- If the Telegram notification fails, the rejection MUST still be persisted. Notification failure MUST be logged but MUST NOT roll back the status change.

---

### REQ-APR-04: Owner Views Expense Detail

**Given** an authenticated owner  
**When** the owner requests a specific expense by `id`  
**Then** the system MUST return the full expense detail including all receipt fields and the linked photo URL

- The system MUST verify `owner_id` match before returning data. An expense belonging to another owner MUST return HTTP 404 Not Found.

---

## 6. Reporting

### REQ-RPT-01: Expense List with Filters

**Given** an authenticated owner  
**When** the owner requests the expense list with optional filters  
**Then** the system MUST return expenses matching ALL supplied filters, scoped to the owner's `owner_id`

Supported filters (all optional, combinable):
- `taxi_id` (UUID) — expenses for a specific taxi
- `driver_id` (UUID) — expenses for a specific driver
- `date_from` / `date_to` (ISO 8601 date) — inclusive date range on `expenses.created_at`
- `category_id` (UUID) — expenses of a specific category
- `status` (enum: `pending`, `confirmed`, `approved`, `rejected`) — expenses in a specific status

- Results MUST be paginated (default: 20, max: 100 per page).
- Results MUST be ordered by `created_at` descending by default.
- An owner MUST NOT receive expenses from other owners regardless of filter values supplied.

---

### REQ-RPT-02: Total Expenses per Taxi per Period

**Given** an authenticated owner  
**When** the owner requests the taxi expense summary for a given date range (`date_from`, `date_to`)  
**Then** the system MUST return an aggregated list: one row per taxi with `taxi_plate`, `total_amount` (sum of `approved` expenses in COP), and `expense_count`

- Only expenses with `status=approved` MUST be included in the totals.
- Taxis with zero approved expenses in the period SHOULD be included with `total_amount=0` and `expense_count=0`.
- Results MUST be scoped to the owner's `owner_id`.

---

### REQ-RPT-03: Total Expenses per Driver per Period

**Given** an authenticated owner  
**When** the owner requests the driver expense summary for a given date range  
**Then** the system MUST return an aggregated list: one row per driver with `driver_name`, `total_amount`, and `expense_count`

- Only expenses with `status=approved` MUST be included in the totals.
- Results MUST be scoped to the owner's `owner_id`.
- Inactive drivers with approved expenses in the period MUST still appear in results.

---

### REQ-RPT-04: Expense Category Breakdown

**Given** an authenticated owner  
**When** the owner requests a category breakdown for a given period  
**Then** the system MUST return one row per category with `category_name`, `total_amount`, and `expense_count` for approved expenses in the period

- Results MUST be scoped to the owner's `owner_id`.

---

## 7. Multi-tenancy Isolation

### REQ-TNT-01: Owner Data Isolation

**Given** two owners (Owner A and Owner B) each with their own taxis, drivers, and expenses  
**When** Owner A makes any API request (list, get, create, update, filter)  
**Then** the system MUST return only records where `owner_id = Owner A's id`

- `owner_id` MUST be sourced exclusively from the authenticated JWT claims in every request. No endpoint MAY accept `owner_id` as a client-supplied parameter.
- All sqlc queries that touch `owners`, `taxis`, `drivers`, `expenses`, `receipts`, or `driver_taxi_assignments` MUST include a `WHERE owner_id = $n` clause or join through an owner-scoped table.
- Integration tests MUST seed two independent owners and assert that no cross-owner data appears in any query path.

---

### REQ-TNT-02: Driver Scope Isolation

**Given** a driver authenticated via the Telegram bot  
**When** the driver queries their expenses or taxi assignments  
**Then** the system MUST scope results to the driver's own `driver_id` AND `owner_id`

- A driver MUST NOT access another driver's expenses, even if both are under the same owner.
- Bot commands that display expense data MUST always filter by both `driver_id` and `owner_id`.

---

### REQ-TNT-03: Cross-Tenant Resource Access Prevention

**Given** Owner A's expense `id=X`  
**When** Owner B's JWT is used to request expense `id=X`  
**Then** the system MUST return HTTP 404 Not Found

- 404 (not 403) MUST be returned to prevent resource enumeration across tenants.
- This applies to all resource types: taxis, drivers, expenses, receipts, assignments.

---

## 8. Fraud Prevention

### REQ-FRD-01: Expense Requires Linked Receipt

**Given** any attempt to create an expense record  
**When** `receipt_id` is null  
**Then** the system MUST reject the operation

- The `expenses.receipt_id` column MUST be defined as `NOT NULL` with a foreign key constraint in the database schema.
- The `expense.Service.Create()` method MUST enforce this at the application layer before the INSERT, returning a domain error if `receipt_id` is not provided.
- No API endpoint or bot flow MAY create an expense without first creating a receipt record.

---

### REQ-FRD-02: Receipt Photo in Persistent Storage

**Given** a driver submits a receipt photo via Telegram  
**When** the bot processes the photo  
**Then** the system MUST download the photo from Telegram and upload it to persistent object storage (GCS or S3-compatible) BEFORE creating any database records

- The `receipts.storage_url` column MUST be `NOT NULL`. A receipt record MUST NOT be inserted without a valid `storage_url`.
- `receipts.telegram_file_id` MUST be stored as a convenience reference only. The system MUST use `storage_url` as the authoritative source for all downstream processing (OCR, admin review).
- If the object storage upload fails, no receipt or expense record MUST be created, and the driver MUST be notified of the failure.

---

### REQ-FRD-03: Expense Approval Requires Confirmed Status

**Given** an expense in any status other than `confirmed`  
**When** an owner attempts to approve or reject it  
**Then** the system MUST reject the operation with HTTP 409 Conflict

- Expenses MUST follow the status state machine: `pending` → `confirmed` → `approved` | `rejected`.
- Backward transitions (e.g., from `approved` back to `pending`) are not permitted and MUST return HTTP 409 Conflict.

---

### REQ-FRD-04: JWT Claims Cannot Be Overridden by Request Params

**Given** a valid JWT for Owner A  
**When** a request includes any `owner_id`, `driver_id`, or `user_id` field in the body or query params  
**Then** the system MUST ignore those parameters and use exclusively the values from the JWT claims

- The auth middleware MUST inject `owner_id`, `user_id`, and `role` into `context.Context`. Handlers MUST read these values from context only.
- This requirement applies to ALL handlers in `internal/httpapi` and ALL bot commands in `internal/telegram`.

---

## Cross-Cutting Requirements

### Authentication

- All REST API endpoints EXCEPT the JWT issuance endpoint MUST require a valid, non-expired JWT in the `Authorization: Bearer <token>` header.
- Requests with a missing or malformed token MUST return HTTP 401 Unauthorized.
- Requests with a valid token but insufficient role MUST return HTTP 403 Forbidden.
- Expired tokens MUST return HTTP 401 Unauthorized (not 403).

### Error Handling

- All API errors MUST return a JSON body `{ "error": "<human-readable message>", "code": "<machine-readable code>" }`.
- Domain validation errors MUST map to HTTP 422. Not-found errors MUST map to HTTP 404. Auth errors MUST map to HTTP 401/403. Conflicts MUST map to HTTP 409. Unexpected server errors MUST map to HTTP 500 and MUST NOT expose internal details.

### Test Coverage

- All domain service packages (`taxi`, `driver`, `expense`, `receipt`, `auth`) MUST have unit test coverage above 80%.
- Multi-tenant isolation MUST be verified by integration tests that seed two owners and assert zero cross-contamination across all query paths.
- The OCR pipeline MUST be tested with a mocked `OCRClient` interface covering success, partial extraction, and total failure scenarios.
- The status state machine transitions for expenses MUST be covered by table-driven unit tests.
