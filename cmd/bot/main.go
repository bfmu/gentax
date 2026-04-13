package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bmunoz/gentax/internal/app"
	"github.com/bmunoz/gentax/internal/config"
	"github.com/bmunoz/gentax/internal/receipt"
	"github.com/bmunoz/gentax/internal/telegram"
	"github.com/google/uuid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gentax bot: config error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run migrations (idempotent — safe to run from multiple processes).
	if err := app.RunMigrations(cfg.DatabaseURL, "./migrations"); err != nil {
		fmt.Fprintf(os.Stderr, "gentax bot: migrations failed: %v\n", err)
		os.Exit(1)
	}

	pool, err := app.ConnectDB(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gentax bot: database connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	deps, err := app.Build(cfg, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gentax bot: wire deps: %v\n", err)
		os.Exit(1)
	}

	// Start OCR worker in background.
	go deps.OCRWorker.Start(ctx)

	// Build Telegram bot.
	botSvc := telegram.Services{
		Auth:       deps.JWTService,
		Driver:     deps.DriverSvc,
		DriverRepo: deps.DriverRepo,
		Expense:    deps.ExpenseSvc,
		Receipt:    deps.ReceiptRepo,
		Storage:    deps.Storage,
	}
	bot, err := telegram.NewBot(cfg.TelegramBotToken, botSvc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gentax bot: create bot: %v\n", err)
		os.Exit(1)
	}

	// Wire OCR notification: when OCR completes (or fails), notify the driver via Telegram.
	deps.Processor.SetNotify(func(ctx context.Context, driverID, receiptID uuid.UUID, result *receipt.OCRResult) error {
		tid, err := deps.DriverRepo.GetDriverTelegramID(ctx, driverID)
		if err != nil || tid == nil {
			return nil
		}
		return bot.NotifyOCRResult(ctx, *tid, receiptID, result)
	})

	slog.Info("gentax bot starting")

	// Run bot in a goroutine; stop on context cancellation.
	go func() {
		<-ctx.Done()
		slog.Info("gentax bot: shutting down")
		bot.Stop()
	}()

	bot.Start() // blocking long-poll loop
	slog.Info("gentax bot: stopped")
}
