package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/Yolanesso/avito-labs/internal/config"
	"github.com/Yolanesso/avito-labs/internal/database"
	"github.com/Yolanesso/avito-labs/internal/httpserver"
	"github.com/Yolanesso/avito-labs/internal/infra/telemetry"
	"github.com/Yolanesso/avito-labs/internal/repository"
	"github.com/Yolanesso/avito-labs/internal/service"
	"github.com/Yolanesso/avito-labs/internal/txmanager"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := telemetry.NewLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	shutdownTracing, err := telemetry.InitTracing(context.Background())
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := shutdownTracing(ctx); err != nil {
			logger.Error("shutdown tracing", "error", err)
		}
	}()

	shutdownMetrics, err := telemetry.InitMetrics(context.Background())
	if err != nil {
		return fmt.Errorf("init metrics: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := shutdownMetrics(ctx); err != nil {
			logger.Error("shutdown metrics", "error", err)
		}
	}()

	connectContext, cancelConnect := context.WithTimeout(
		context.Background(),
		cfg.Database.ConnectTimeout,
	)

	pool, err := database.NewPool(connectContext, cfg.Database)
	cancelConnect()
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	logger.Info("connected to PostgreSQL")

	tripRepository := repository.NewTripRepository(
		pool,
		cfg.Database.QueryTimeout,
	)
	transactionManager := txmanager.New(pool)
	tripService := service.NewTripService(
		tripRepository,
		transactionManager,
	)

	tripMetrics, err := telemetry.NewTripMetrics(logger)
	if err != nil {
		return fmt.Errorf("init trip metrics: %w", err)
	}
	tripService.SetMetrics(tripMetrics)

	handler := httpserver.NewHandler(
		pool.Ping,
		cfg.Database.QueryTimeout,
		tripService,
	)
	server := httpserver.New(cfg.HTTPAddr, handler, logger)

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("trip-service listening", "addr", cfg.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	signalContext, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server: %w", err)
		}

	case <-signalContext.Done():
		logger.Info("shutdown signal received")

		shutdownContext, cancelShutdown := context.WithTimeout(
			context.Background(),
			cfg.ShutdownTimeout,
		)
		defer cancelShutdown()

		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}

	logger.Info("trip-service stopped")
	return nil
}
