package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pb "github.com/ArmanAA/rain-staking/gen/staking/v1"
	"github.com/ArmanAA/rain-staking/config"
	grpcadapter "github.com/ArmanAA/rain-staking/internal/adapter/grpc"
	"github.com/ArmanAA/rain-staking/internal/adapter/bitgo"
	"github.com/ArmanAA/rain-staking/internal/adapter/mock"
	"github.com/ArmanAA/rain-staking/internal/adapter/postgres"
	"github.com/ArmanAA/rain-staking/internal/port"
	"github.com/ArmanAA/rain-staking/internal/service"
	"github.com/ArmanAA/rain-staking/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		slog.Error("invalid log level, defaulting to info", slog.String("configured", cfg.LogLevel))
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Database connection pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("connected to database")

	// Repositories
	stakeRepo := postgres.NewStakeRepo(pool)
	balanceRepo := postgres.NewBalanceRepo(pool)
	rewardRepo := postgres.NewRewardRepo(pool)
	auditRepo := postgres.NewAuditEventRepo(pool)

	// Event publisher
	publisher := postgres.NewLogEventPublisher(auditRepo, logger)

	// Staking provider (BitGo or Mock based on config)
	var stakingProvider port.StakingProvider
	switch cfg.StakingProvider {
	case "bitgo":
		stakingProvider = bitgo.NewClient(
			cfg.BitGoBaseURL, cfg.BitGoAccessToken,
			cfg.BitGoWalletID, cfg.BitGoCoin, logger,
		)
		logger.Info("using BitGo staking provider", slog.String("base_url", cfg.BitGoBaseURL))
	default:
		stakingProvider = mock.NewStakingProvider(logger)
		logger.Info("using mock staking provider")
	}

	// Application services
	stakingSvc := service.NewStakingService(stakeRepo, balanceRepo, stakingProvider, publisher, logger)
	balanceSvc := service.NewBalanceService(balanceRepo, publisher)
	rewardSvc := service.NewRewardService(rewardRepo)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcadapter.RecoveryInterceptor(logger),
			grpcadapter.LoggingInterceptor(logger),
			grpcadapter.AuthInterceptor(cfg.JWTSecret),
			grpcadapter.ValidationInterceptor(),
		),
	)
	handler := grpcadapter.NewStakingHandler(stakingSvc, balanceSvc, rewardSvc)
	pb.RegisterStakingServiceServer(grpcServer, handler)
	reflection.Register(grpcServer) // Enables grpcurl introspection

	// Start gRPC server
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		logger.Error("failed to listen for gRPC", slog.String("error", err.Error()))
		os.Exit(1)
	}
	go func() {
		logger.Info("gRPC server started", slog.Int("port", cfg.GRPCPort))
		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Error("gRPC server failed", slog.String("error", err.Error()))
		}
	}()

	// HTTP gateway (REST proxy that dials back to gRPC server, ensuring
	// all requests pass through the full interceptor chain including auth)
	mux := runtime.NewServeMux()
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := pb.RegisterStakingServiceHandlerFromEndpoint(ctx, mux, fmt.Sprintf("localhost:%d", cfg.GRPCPort), dialOpts); err != nil {
		logger.Error("failed to register HTTP gateway", slog.String("error", err.Error()))
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: mux,
	}
	go func() {
		logger.Info("HTTP gateway started", slog.Int("port", cfg.HTTPPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", slog.String("error", err.Error()))
		}
	}()

	// Reward poller (background worker)
	rewardPoller := worker.NewRewardPoller(
		stakeRepo, balanceRepo, rewardRepo,
		stakingProvider, publisher,
		cfg.RewardPollInterval, logger,
	)
	go rewardPoller.Start(ctx)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info("received shutdown signal", slog.String("signal", sig.String()))

	cancel() // Stop reward poller
	grpcServer.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)

	logger.Info("server shutdown complete")
}
