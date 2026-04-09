# Spec: owner-web-panel

All amounts in COP. All data scoped to `owner_id`. Web routes at `/web/` prefix.

---

## Domain: owner-auth

### Requirement: Owner Bootstrap Registration

The system MUST allow creation of the first owner via a secret-gated endpoint. `BOOTSTRAP_SECRET` env var MUST be validated; missing or wrong secret MUST return 403. Subsequent owners MUST be created by an authenticated owner.

#### Scenario: Successful bootstrap
- GIVEN `BOOTSTRAP_SECRET` is set and no owners exist
- WHEN `POST /web/auth/bootstrap` with matching secret, name, email, password
- THEN owner created with bcrypt-hashed password; redirect to login

#### Scenario: Invalid secret
- GIVEN wrong or missing secret
- WHEN `POST /web/auth/bootstrap`
- THEN 403 returned; no owner created

### Requirement: Owner Login

The system MUST authenticate owners via email + bcrypt password. On success, MUST set an `httpOnly; SameSite=Lax` session cookie containing an 8h JWT (`role=admin`).

#### Scenario: Successful login
- GIVEN valid email and password
- WHEN `POST /web/auth/login`
- THEN session cookie set; redirect to `/web/`

#### Scenario: Invalid credentials
- GIVEN wrong email or password
- WHEN `POST /web/auth/login`
- THEN form re-rendered with error; no cookie set

### Requirement: Web Session Validation

Web routes MUST extract the JWT from the session cookie. Missing, malformed, or expired cookie MUST redirect to `/web/login`.

---

## Domain: webpanel-core

### Requirement: Dashboard Summary

`GET /web/` MUST display three counts scoped to the authenticated owner: active taxis, active drivers, expenses pending review (status=confirmed).

#### Scenario: Counts are owner-scoped
- GIVEN owner with 3 active taxis, 5 active drivers, 2 confirmed expenses
- WHEN `GET /web/`
- THEN dashboard shows exactly those counts; other owners' data not included

---

## Domain: owner-fleet

### Requirement: Taxi Management via Web

The web panel MUST allow listing all taxis and creating new taxis via HTML form. Validation rules from REQ-TAX-01 apply (plate uniqueness, year range).

#### Scenario: Create taxi
- GIVEN authenticated owner
- WHEN valid plate, model, year submitted
- THEN taxi created; list reloads

#### Scenario: Duplicate plate
- GIVEN plate already exists for this owner
- WHEN form submitted
- THEN form re-rendered with conflict error; no record created

### Requirement: Driver Management and Link Token via Web

The web panel MUST allow listing drivers, creating drivers, and generating a Telegram link token per driver. The generated link MUST be displayed as a `t.me/<bot>?start=<token>` URL the owner can share.

#### Scenario: Generate link token
- GIVEN existing driver without telegram_id
- WHEN owner clicks "Generate link"
- THEN single-use 24h token created; full bot deep-link URL displayed on page

---

## Domain: owner-expenses

### Requirement: Expense Review Queue

The web panel MUST list expenses with status=confirmed, ordered by created_at ascending. Each row MUST show driver name, taxi plate, category, amount, and receipt thumbnail.

#### Scenario: Approve via HTMX
- GIVEN expense in status=confirmed
- WHEN owner clicks "Approve"
- THEN `PATCH /web/expenses/{id}/approve` sets status=approved; row removed without full page reload

#### Scenario: Reject with reason
- GIVEN expense in status=confirmed
- WHEN owner submits rejection with optional reason
- THEN status=rejected; driver notified via Telegram per REQ-APR-03

#### Scenario: View expense detail
- GIVEN expense with receipt
- WHEN `GET /web/expenses/{id}`
- THEN receipt image, OCR extracted fields, and approve/reject controls displayed

---

## Domain: owner-reports

### Requirement: Expense Summary by Taxi and Driver

The reports page MUST display approved-expense totals grouped by taxi and by driver for a date range supplied by the owner.

#### Scenario: Date-filtered report
- GIVEN owner provides date_from and date_to
- WHEN form submitted
- THEN page shows per-taxi totals and per-driver totals in COP, scoped to owner_id

---

## Domain: owner-domain

### Requirement: Owner Entity and Repository

The system MUST have an `internal/owner/` package: Owner entity (id, name, email, password_hash, created_at), OwnerRepository interface (GetByEmail, Create), and OwnerService (Authenticate via bcrypt, Create).

#### Scenario: Authenticate — correct password
- GIVEN stored bcrypt hash for owner email
- WHEN Authenticate(email, plaintext) called
- THEN owner returned

#### Scenario: Authenticate — wrong password
- GIVEN stored bcrypt hash
- WHEN Authenticate called with wrong password
- THEN ErrInvalidCredentials returned

---

## Domain: api-middleware (delta)

### ADDED: Cookie Session Auth for Web Routes

Web panel routes (`/web/*`) MUST read the JWT from an `httpOnly` session cookie. The existing `Authorization: Bearer` header auth MUST remain unchanged for all REST API routes. The JSON Content-Type middleware MUST be scoped to API routes only and MUST NOT apply to `/web/*` routes.
