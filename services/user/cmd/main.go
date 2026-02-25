package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/AlfariziYasir/transactions/common/pkg/httpx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/middleware"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/common/pkg/redis"
	"github.com/AlfariziYasir/transactions/common/proto/user"
	"github.com/AlfariziYasir/transactions/services/user/config"
	"github.com/AlfariziYasir/transactions/services/user/internal/adapters/handler"
	"github.com/AlfariziYasir/transactions/services/user/internal/adapters/repository"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/services"
	"github.com/AlfariziYasir/transactions/services/user/migrations"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal(err)
		return
	}

	l, err := logger.New(cfg.LogLevel, cfg.Name, cfg.Version)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer l.Logger.Sync()

	pg, err := postgres.New(context.Background(), cfg.DbDsn, l.Logger)
	if err != nil {
		l.Fatal("failed connect to postgres db", zap.Error(err))
		return
	}
	defer pg.Close()

	err = migrations.RunMigrations(cfg.DbDsn)
	if err != nil {
		l.Logger.Fatal("failed to run migrations", zap.Error(err))
	}

	rds, err := redis.NewRedisCache(cfg.RedisAddress, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		l.Fatal("failed connect to redis", zap.Error(err))
		return
	}

	repo := repository.NewRepository(rds, pg.Pool)
	svc := services.NewUserService(cfg, l, repo, rds)
	userHandler := handler.NewHandler(l, svc)

	authInterceptor := middleware.NewAuthInterceptor(l, rds)
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			authInterceptor.Unary(cfg.AccessTokenKey, cfg.RefreshTokenKey),
		),
	)

	errChan := make(chan error, 1)
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GrpcPort))
		if err != nil {
			errChan <- err
			return
		}

		user.RegisterUserServiceServer(grpcServer, userHandler)
		l.Info("grpc server starting", zap.Int("port", cfg.GrpcPort))
		err = grpcServer.Serve(lis)
		if err != nil {
			errChan <- err

		}
	}()

	go func() {
		err := httpx.RunGateway(
			ctx,
			cfg.GrpcPort,
			cfg.HtppPort,
			l,
			user.RegisterUserServiceHandlerFromEndpoint,
			cfg.SwaggerPath,
		)
		if err != nil {
			errChan <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case <-quit:
		l.Info("shutting down servers")
		cancel()
		grpcServer.GracefulStop()
		l.Info("server stopped safely")
	case err := <-errChan:
		l.Fatal("server error", zap.Error(err))
	}
}
