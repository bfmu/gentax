package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bmunoz/gentax/internal/app"
	"github.com/bmunoz/gentax/internal/config"
	"github.com/bmunoz/gentax/internal/httpapi"
)

func main() {
	migrateUp := flag.Bool("migrate-up", false, "run database migrations up and exit")
	migrateDown := flag.Bool("migrate-down", false, "run database migrations down and exit (not implemented)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gentax api: config error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if *migrateDown {
		fmt.Fprintln(os.Stderr, "migrate-down: not implemented")
		os.Exit(1)
	}

	// Always run migrations on startup (idempotent).
	if err := app.RunMigrations(cfg.DatabaseURL, "./migrations"); err != nil {
		fmt.Fprintf(os.Stderr, "gentax api: migrations failed: %v\n", err)
		os.Exit(1)
	}

	if *migrateUp {
		slog.Info("migrations complete, exiting (--migrate-up flag)")
		os.Exit(0)
	}

	pool, err := app.ConnectDB(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gentax api: database connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	deps, err := app.Build(cfg, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gentax api: wire deps: %v\n", err)
		os.Exit(1)
	}

	// Start OCR worker in background.
	go deps.OCRWorker.Start(ctx)

	// Build HTTP router.
	svc := httpapi.Services{
		Auth:         deps.JWTService,
		DriverFinder: deps.DriverRepo,
		Taxi:         deps.TaxiSvc,
		Driver:       deps.DriverSvc,
		Expense:      deps.ExpenseSvc,
	}
	handler := httpapi.NewRouter(svc, deps.JWTService)

	addr := fmt.Sprintf(":%d", cfg.APIPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("gentax api starting", "addr", addr, "env", cfg.AppEnv)

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		fmt.Fprintf(os.Stderr, "gentax api: server error: %v\n", err)
		os.Exit(1)
	case <-ctx.Done():
		slog.Info("gentax api: shutting down")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("gentax api: graceful shutdown failed", "error", err)
	}
	slog.Info("gentax api: stopped")
}
