package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcserver "github.com/AlfariziYasir/transactions/common/pkg/grpc-server"
	httpserver "github.com/AlfariziYasir/transactions/common/pkg/http-server"
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
	"github.com/flowchartsman/swaggerui"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

//go:embed user_api.swagger.json
var spec []byte

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

	repo := repository.NewRepository(pg.Pool)
	svc := services.NewUserService(cfg, l, repo, rds)
	userHandler := handler.NewHandler(l, svc)

	authInterceptor := middleware.NewAuthInterceptor(l, rds)
	serverOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(
			authInterceptor.Unary(cfg.AccessTokenKey, cfg.RefreshTokenKey),
		),
	}

	// start grpc server
	grpcServer, err := grpcserver.New(uint32(cfg.GrpcPort), serverOptions...)
	if err != nil {
		l.Fatal("failed to create new gateway grpc server", zap.Error(err))
		return
	}

	user.RegisterUserServiceServer(grpcServer.Server, userHandler)
	reflection.Register(grpcServer.Server)
	if err = grpcServer.Start(); err != nil {
		l.Fatal("failed to start gateway grpcServer", zap.Error(err))
		return
	}
	l.Info("GRPC server started")

	// start http server
	httpMux := http.NewServeMux()
	gwmux := grpcserver.RuntimeServer()
	err = user.RegisterUserServiceHandlerFromEndpoint(
		ctx,
		gwmux,
		fmt.Sprintf("%s:%d", cfg.GrpcHost, cfg.GrpcPort),
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	)
	if err != nil {
		l.Fatal("failed to register api gateway handler from endpoint", zap.Error(err))
		return
	}

	httpMux.Handle("/docs/", http.StripPrefix("/docs", swaggerui.Handler(spec)))
	httpMux.Handle("/", gwmux)
	httpServer := httpserver.New(
		httpserver.AllowCors(httpMux),
		httpserver.Port(fmt.Sprintf("%d", cfg.HttpPort)),
		httpserver.ReadTimeout(10*time.Second),
		httpserver.WriteTimeout(10*time.Second),
		httpserver.ShutdownTimeout(5*time.Second),
	)

	go httpServer.Start()
	l.Info("HTTP server started")

	// Waiting signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	select {
	case s := <-interrupt:
		l.Info("app - Run - signal", zap.String("signal", s.String()))
	case err = <-grpcServer.Notify():
		l.Error("app - Run - grpcServer.Notify", zap.Error(err))
	case err = <-httpServer.Notify():
		l.Error("app - Run - httpServer.Notify", zap.Error(err))
	}

	l.Info("shutting down servers...")
	cancel()
	time.Sleep(1 * time.Second)
	if err := grpcServer.Shutdown(); err != nil {
		l.Error("failed to shutdown grpc server gracefully", zap.Error(err))
	}
	if err := httpServer.Shutdown(); err != nil {
		l.Error("failed to shutdown http server gracefully", zap.Error(err))
	}
	l.Info("server exited properly")
}
