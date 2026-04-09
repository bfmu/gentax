// Package app provides the shared dependency-wiring helper used by both cmd/api and cmd/bot.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bmunoz/gentax/internal/auth"
	"github.com/bmunoz/gentax/internal/config"
	"github.com/bmunoz/gentax/internal/driver"
	"github.com/bmunoz/gentax/internal/expense"
	"github.com/bmunoz/gentax/internal/receipt"
	"github.com/bmunoz/gentax/internal/storage"
	"github.com/bmunoz/gentax/internal/taxi"
	"github.com/bmunoz/gentax/internal/worker"
)

// Deps holds all wired application dependencies.
type Deps struct {
	Pool       *pgxpool.Pool
	JWTService *auth.JWTService

	TaxiRepo    taxi.Repository
	DriverRepo  driver.Repository
	ExpenseRepo expense.Repository
	ReceiptRepo receipt.Repository

	TaxiSvc    taxi.Service
	DriverSvc  driver.Service
	ExpenseSvc expense.Service

	Storage   receipt.StorageClient
	Processor receipt.Processor
	OCRWorker *worker.OCRWorker
}

// Build constructs all repositories, services, and the OCR worker from the given config and pool.
func Build(cfg *config.Config, pool *pgxpool.Pool) (*Deps, error) {
	// Repositories
	taxiRepo := taxi.NewRepository(pool)
	driverRepo := driver.NewRepository(pool)
	expenseRepo := expense.NewRepository(pool)
	receiptRepo := receipt.NewRepository(pool)

	// Services
	taxiSvc := taxi.NewService(taxiRepo)
	driverSvc := driver.NewService(driverRepo)
	expenseSvc := expense.NewService(expenseRepo)

	// JWT
	jwtSvc := auth.NewJWTService(cfg.JWTSecret)

	// Local storage
	storageClient, err := storage.NewLocalStorageClient("./data/receipts")
	if err != nil {
		return nil, fmt.Errorf("wire: local storage: %w", err)
	}

	// OCR client — Tesseract when provider is "tesseract" (or empty), otherwise stub
	var ocrClient receipt.OCRClient
	if cfg.OCRProvider == "tesseract" || cfg.OCRProvider == "" {
		ocrClient = receipt.NewTesseractClient()
	} else {
		// Future: wire google_vision / openai clients here.
		// For now fall back to Tesseract.
		ocrClient = receipt.NewTesseractClient()
	}

	// OCR processor (wired without a notify func; wire notify after bot is constructed if needed)
	processor := receipt.NewProcessor(receiptRepo, ocrClient, storageClient, nil)

	// OCR worker
	ocrWorker := worker.NewOCRWorker(
		receiptRepo,
		processor,
		cfg.OCRWorkerPoolSize,
		time.Duration(cfg.OCRPollIntervalSecs)*time.Second,
	)

	return &Deps{
		Pool:       pool,
		JWTService: jwtSvc,

		TaxiRepo:    taxiRepo,
		DriverRepo:  driverRepo,
		ExpenseRepo: expenseRepo,
		ReceiptRepo: receiptRepo,

		TaxiSvc:    taxiSvc,
		DriverSvc:  driverSvc,
		ExpenseSvc: expenseSvc,

		Storage:   storageClient,
		Processor: processor,
		OCRWorker: ocrWorker,
	}, nil
}

// ConnectDB opens a pgxpool connection to the given database URL and pings it.
func ConnectDB(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}

// RunMigrations applies all pending up migrations from migrationsDir using the given DB URL.
func RunMigrations(databaseURL, migrationsDir string) error {
	// golang-migrate pgx/v5 driver registers under "pgx5" scheme.
	migrateURL := databaseURL
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if len(databaseURL) >= len(prefix) && databaseURL[:len(prefix)] == prefix {
			migrateURL = "pgx5://" + databaseURL[len(prefix):]
			break
		}
	}

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsDir),
		migrateURL,
	)
	if err != nil {
		return fmt.Errorf("migrations: create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrations: up: %w", err)
	}
	slog.Info("migrations: applied successfully")
	return nil
}
