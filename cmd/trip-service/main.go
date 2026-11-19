package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Yolanesso/avito-labs/internal/config"
	"github.com/Yolanesso/avito-labs/internal/database"
	"github.com/Yolanesso/avito-labs/internal/gateway/push"
	"github.com/Yolanesso/avito-labs/internal/httpserver"
	positionhttp "github.com/Yolanesso/avito-labs/internal/httpserver/trip/positions"
	"github.com/Yolanesso/avito-labs/internal/infra/telemetry"
	"github.com/Yolanesso/avito-labs/internal/repository"
	"github.com/Yolanesso/avito-labs/internal/service"
	"github.com/Yolanesso/avito-labs/internal/txmanager"
	positionusecase "github.com/Yolanesso/avito-labs/internal/usecase/trip/positions"
	positionworker "github.com/Yolanesso/avito-labs/internal/worker/positions"
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
	slog.SetDefault(logger)

	// Один бюджет на все этапы остановки, включая выгрузку телеметрии.
	var shutdownDeadline time.Time
	shutdown := func() (context.Context, context.CancelFunc) {
		if shutdownDeadline.IsZero() {
			shutdownDeadline = time.Now().Add(cfg.ShutdownTimeout)
		}
		return context.WithDeadline(context.Background(), shutdownDeadline)
	}

	shutdownTracing, err := telemetry.InitTracing(context.Background())
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}
	defer func() {
		ctx, cancel := shutdown()
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
		ctx, cancel := shutdown()
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
	activeRegistration, err := tripMetrics.ObserveActive(tripRepository)
	if err != nil {
		return fmt.Errorf("observe active trips: %w", err)
	}
	defer func() {
		if err := activeRegistration.Unregister(); err != nil {
			logger.Error("unregister active trips metric", "error", err)
		}
	}()

	pushClient, err := push.New(cfg.Positions.PushHTTPURL, cfg.Positions.RequestTimeout)
	if err != nil {
		return fmt.Errorf("init push client: %w", err)
	}
	defer pushClient.Close()
	positionMetrics, err := telemetry.NewPositionMetrics()
	if err != nil {
		return fmt.Errorf("init position metrics: %w", err)
	}
	ticker := positionworker.NewTicker(cfg.Positions.CheckInterval)
	defer ticker.Stop()
	positions := positionworker.New(positionworker.Config{
		StaleAfter: cfg.Positions.StaleAfter, Workers: cfg.Positions.Workers,
		QueueSize: cfg.Positions.QueueSize, RequestTimeout: cfg.Positions.RequestTimeout,
		BatchSize: cfg.Positions.BatchSize,
	}, tripRepository, pushClient, positionMetrics, logger,
		ticker)
	registration, err := positionMetrics.ObservePool(positions.Stats)
	if err != nil {
		return err
	}
	defer func() {
		if err := registration.Unregister(); err != nil {
			logger.Error("unregister position metrics", "error", err)
		}
	}()
	positions.Start(context.Background())

	handler := httpserver.NewHandler(
		pool.Ping,
		cfg.Database.QueryTimeout,
		tripService,
	)
	handler.SetPositions(positionhttp.New(
		positionusecase.New(tripRepository, transactionManager, positions, positionMetrics),
		time.Now,
	))
	server := httpserver.New(cfg.HTTPAddr, handler, logger, httpserver.Timeouts{
		ReadTimeout: cfg.HTTPTimeouts.ReadTimeout, ReadHeaderTimeout: cfg.HTTPTimeouts.ReadHeaderTimeout,
		WriteTimeout: cfg.HTTPTimeouts.WriteTimeout, IdleTimeout: cfg.HTTPTimeouts.IdleTimeout,
	})

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

	var serverErr error
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			serverErr = fmt.Errorf("HTTP server: %w", err)
		}

	case <-signalContext.Done():
		logger.Info("shutdown signal received")
	}

	shutdownContext, cancelShutdown := shutdown()
	defer cancelShutdown()
	// Порядок важен: HTTP -> сканер -> очередь и воркеры -> клиенты и БД (defer).
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("HTTP shutdown deadline exceeded", "error", err)
		serverErr = errors.Join(serverErr, err)
		if closeErr := server.Close(); closeErr != nil {
			logger.Error("force close HTTP server", "error", closeErr)
		}
	}
	if err := positions.Shutdown(shutdownContext); err != nil {
		logger.Error("position shutdown deadline exceeded", "error", err)
		serverErr = errors.Join(serverErr, err)
	}

	logger.Info("trip-service stopped")
	return serverErr
}
