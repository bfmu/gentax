# Change Proposal: owner-web-panel

## Intent
Taxi fleet owners have no way to manage their vehicles, drivers, or expenses — all administration currently requires direct database access. This change delivers a server-rendered web panel so owners can self-serve fleet operations from a browser.

## Context
The REST API serves the driver mobile app but exposes no owner-facing UI. The `owners` table exists with basic fields (id, name, email, created_at) but has no authentication support. There is no `internal/owner/` package — owner domain logic must be built from scratch. The JSON Content-Type middleware is applied globally, which would break HTML responses.

## Approach
- **Auth**: Add `password_hash` column to `owners` (migration 000008). Use bcrypt via `golang.org/x/crypto` (already a transitive dep). Issue the same HS256 JWT with `role=admin` and `OwnerID` claim; store it in an httpOnly session cookie. 8h TTL for web sessions. Bootstrap first owner via env-secret-protected endpoint; subsequent owners created by existing owners.
- **Frontend**: HTMX + Go `html/template`, served from the same binary. New `internal/webpanel/` package with its own Chi sub-router mounted at `/web/`. Thin cookie-reading middleware adapts the existing JWT auth. Base layout with nav bar shared across 7 MVP screens.
- **Integration**: Web panel handlers call domain services directly (same Go interfaces as REST handlers) — no HTTP round-trip. `app.Build()` wires both the API and web routers. Move JSON Content-Type middleware from global scope to API route group only.

## Capabilities

### New Capabilities
- **owner-auth**: Owner login (email+password), session cookie, bootstrap registration endpoint
- **owner-fleet**: List/create taxis; list/create drivers; generate driver link tokens
- **owner-expenses**: List pending expenses; view detail with receipt image and OCR text; approve/reject via HTMX
- **owner-reports**: Expense totals by taxi/driver with date-range filter
- **webpanel-core**: `/web/` Chi sub-router, cookie session middleware, HTMX base layout, login/dashboard screens

### Modified Capabilities
- **owner-domain**: New `internal/owner/` package — entity, repository interface, Postgres repo, service layer
- **api-middleware**: Move JSON Content-Type middleware from global to API-only route group

## Out of Scope
- Password reset / SMTP integration
- Owner self-registration (admin-only creation)
- Excel/CSV export
- Superadmin role or multi-tenant isolation
- Owner profile editing
- Driver mobile app changes

## Risks
| Risk | Impact | Mitigation |
|------|--------|------------|
| `chi` is indirect dep — import may break build | Build failure | Run `go mod tidy` immediately after first direct import |
| Global JSON middleware breaks HTML responses | Web panel unusable | Move middleware to API group in first PR before any web routes |
| Cookie auth introduces CSRF vector | Security | Use SameSite=Lax + CSRF token on mutation forms |
| Bootstrap endpoint exposed in production | Unauthorized owner creation | Gate behind env secret; disable after first owner exists |
| HTMX partial responses mixed with full pages | Broken UI on direct navigation | Detect `HX-Request` header; return full page or partial accordingly |
