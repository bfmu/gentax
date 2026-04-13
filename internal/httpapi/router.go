// Package httpapi wires the Chi router, middleware stack, and all HTTP handlers.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/httpapi/handlers"
	mw "github.com/bmunoz/gentax/internal/httpapi/middleware"
	"github.com/bmunoz/gentax/internal/owner"
	"github.com/bmunoz/gentax/internal/taxi"
)

// Services holds all domain service dependencies needed by the HTTP layer.
type Services struct {
	Auth              auth.TokenValidator
	DriverFinder      handlers.DriverFinder // repository-level finder for auth bootstrap
	Taxi              taxi.Service
	Driver            driver.Service
	Expense           expense.Service
	Owner             owner.Service
	EvidenceNotifier  handlers.EvidenceNotifier // optional; nil disables Telegram notifications
	StorageReader     handlers.StorageReader    // optional; nil disables receipt proxy
	BootstrapSecret   string
	CORSOrigin        string
}

// NewRouter builds and returns the Chi router with all routes mounted.
//
// Public routes (no auth required):
//   - POST /auth/telegram
//
// Protected routes (require valid JWT, role=admin):
//   - GET/POST   /taxis
//   - DELETE     /taxis/{id}
//   - POST       /taxis/{id}/assign/{driverID}
//   - DELETE     /taxis/{id}/assign/{driverID}
//   - GET/POST   /drivers
//   - DELETE     /drivers/{id}
//   - POST       /drivers/{id}/link-token
//   - GET        /expenses
//   - GET        /expenses/{id}
//   - GET        /expenses/{id}/attachments
//   - PATCH      /expenses/{id}/approve
//   - PATCH      /expenses/{id}/reject
//   - GET        /reports/expenses
//   - GET        /reports/taxis
//   - GET        /reports/drivers
//   - GET        /reports/categories
//
// Protected routes (require valid JWT, role=driver):
//   - POST /expenses
//   - POST /expenses/{id}/attachments
func NewRouter(svc Services, issuer auth.TokenIssuer) http.Handler {
	r := chi.NewRouter()

	// Global middleware: logging, panic recovery, CORS.
	r.Use(mw.RequestLogger)
	r.Use(chimw.Recoverer)
	r.Use(mw.CORS(svc.CORSOrigin))

	// Handler instances.
	authH := handlers.NewAuthHandler(svc.DriverFinder, issuer)
	ownerAuthH := handlers.NewOwnerAuthHandler(svc.Owner, issuer, svc.BootstrapSecret).WithExpenseService(svc.Expense)
	taxiH := handlers.NewTaxiHandler(svc.Taxi, svc.Driver)
	driverH := handlers.NewDriverHandler(svc.Driver)
	expenseH := handlers.NewExpenseHandler(svc.Expense).
		WithEvidenceNotifier(svc.EvidenceNotifier).
		WithStorageReader(svc.StorageReader)
	reportH := handlers.NewReportHandler(svc.Expense)
	catH := handlers.NewCategoryHandler(svc.Expense)

	// Public routes — no JWT required.
	r.Post("/auth/telegram", authH.TelegramAuth)
	r.Post("/auth/owner/login", ownerAuthH.Login)
	r.Post("/auth/owner/bootstrap", ownerAuthH.Bootstrap)
	r.Post("/auth/owner/register", ownerAuthH.Register)

	// Admin-protected routes (role=admin).
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(svc.Auth, auth.RoleAdmin))

		// Taxi management.
		r.Get("/taxis", taxiH.List)
		r.Post("/taxis", taxiH.Create)
		r.Delete("/taxis/{id}", taxiH.Deactivate)
		r.Post("/taxis/{id}/assign/{driverID}", taxiH.AssignDriver)
		r.Delete("/taxis/{id}/assign/{driverID}", taxiH.UnassignDriver)

		// Driver management.
		r.Get("/drivers", driverH.List)
		r.Post("/drivers", driverH.Create)
		r.Delete("/drivers/{id}", driverH.Deactivate)
		r.Post("/drivers/{id}/link-token", driverH.GenerateLinkToken)

		// Expense management (admin operations).
		r.Get("/expenses", expenseH.List)
		r.Get("/expenses/{id}", expenseH.GetByID)
		r.Get("/expenses/{id}/receipt", expenseH.ReceiptProxy)
		r.Get("/expenses/{id}/attachments", expenseH.ListAttachments)
		r.Patch("/expenses/{id}/approve", expenseH.Approve)
		r.Patch("/expenses/{id}/reject", expenseH.Reject)
		r.Patch("/expenses/{id}/request-evidence", expenseH.RequestEvidence)

		// Category management.
		r.Get("/categories", catH.List)
		r.Post("/categories", catH.Create)
		r.Delete("/categories/{id}", catH.Delete)

		// Reports.
		r.Get("/reports/expenses", reportH.ExpenseList)
		r.Get("/reports/taxis", reportH.TaxiSummary)
		r.Get("/reports/drivers", reportH.DriverSummary)
		r.Get("/reports/categories", reportH.CategorySummary)
	})

	// Driver-protected routes (role=driver).
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(svc.Auth, auth.RoleDriver))

		// Expense submission.
		r.Post("/expenses", expenseH.Create)
		// Attachment submission (driver adds extra evidence photos).
		r.Post("/expenses/{id}/attachments", expenseH.AddAttachment)
	})

	return r
}
