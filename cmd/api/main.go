package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/app"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	"github.com/diagnosis/luxsuv-api-v2/internal/routes"
	"github.com/diagnosis/luxsuv-api-v2/internal/store"
	"github.com/diagnosis/luxsuv-api-v2/migrations"
)

func main() {

	ctx := context.Background()
	logger.Info(ctx, "starting application")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error(ctx, "DATABASE_URL environment variable is required")
		os.Exit(1)
	}

	pool, err := store.OpenPool(dsn)
	if err != nil {
		logger.Error(ctx, "failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info(ctx, "database connection established")

	if err = store.MigrateFS(dsn, migrations.FS, "."); err != nil {
		pool.Close()
		logger.Error(ctx, "database migration failed", "error", err)
		os.Exit(1)
	}
	logger.Info(ctx, "database migrations applied successfully")

	appl, err := app.NewApplication(pool)
	if err != nil {
		pool.Close()
		logger.Error(ctx, "application initialization failed", "error", err)
		os.Exit(1)
	}

	r := routes.SetRouter(appl)

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	serv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
	}

	errch := make(chan error, 1)
	go func() {
		domain := os.Getenv("APP_DOMAIN")
		if domain == "" {
			domain = "localhost"
		}
		appURL := fmt.Sprintf("http://%s:%s", domain, port)
		logger.Info(ctx, "server starting", "url", appURL, "port", port)
		errch <- serv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case <-stop:
		logger.Info(ctx, "shutdown signal received, shutting down gracefully")
	case err := <-errch:
		if err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "server error", "error", err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := serv.Shutdown(ctx); err != nil {
		logger.Error(ctx, "graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info(ctx, "server stopped successfully")
}
