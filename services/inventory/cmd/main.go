package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net"
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
	"github.com/AlfariziYasir/transactions/common/proto/inventory"
	"github.com/AlfariziYasir/transactions/services/inventory/config"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/adapters/handler"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/adapters/repository"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/services"
	"github.com/AlfariziYasir/transactions/services/inventory/migrations"
	"github.com/flowchartsman/swaggerui"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

//go:embed inventory_api.swagger.json
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

	rmqConn, err := amqp091.DialConfig(
		cfg.RabbitMQUrl,
		amqp091.Config{
			Heartbeat: 10 * time.Second,
			Properties: amqp091.Table{
				"product":     cfg.Name,
				"version":     cfg.Version,
				"description": "handles all in and out event at inventory service",
			},
			Dial: func(network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, addr, 5*time.Second)
			},
		},
	)
	if err != nil {
		l.Fatal("failed to connect rabbitmq", zap.Error(err))
		return
	}
	defer rmqConn.Close()

	productRepo := repository.NewProductRepo(pg.Pool)
	stockRepo := repository.NewStockRepo(pg.Pool)
	outboxRepo := repository.NewOutboxRepo(pg.Pool)
	inboxRepo := repository.NewInboxRepo(pg.Pool)
	trx := postgres.NewTransaction(pg.Pool)

	outboxTrigger := make(chan struct{})
	productSvc := services.NewProductServices(productRepo, trx, l)
	stockSvc := services.NewStockService(stockRepo, inboxRepo, trx, l)

	consumerCh, err := rmqConn.Channel()
	if err != nil {
		l.Fatal("channel failed", zap.Error(err))
		return
	}
	defer consumerCh.Close()

	go func() {
		err = handler.NewStockConsumer(stockSvc, consumerCh, l).Start()
		if err != nil {
			l.Fatal("failed to start inventory consumer", zap.Error(err))
			return
		}
	}()

	publisherCh, err := rmqConn.Channel()
	if err != nil {
		l.Fatal("channel failed", zap.Error(err))
		return
	}
	defer publisherCh.Close()

	pub, err := handler.NewPublisher(outboxRepo, publisherCh, l, outboxTrigger)
	if err != nil {
		l.Fatal("failed to start inventory publisher", zap.Error(err))
		return
	}
	go pub.Start(ctx)

	inventoryHandler := handler.NewHandler(productSvc, stockSvc, l)
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

	inventory.RegisterInventoryServiceServer(grpcServer.Server, inventoryHandler)
	reflection.Register(grpcServer.Server)
	if err = grpcServer.Start(); err != nil {
		l.Fatal("failed to start gateway grpcServer", zap.Error(err))
		return
	}
	l.Info("GRPC server started")

	// start http server
	gwmux := grpcserver.RuntimeServer()
	err = inventory.RegisterInventoryServiceHandlerFromEndpoint(
		ctx,
		gwmux,
		fmt.Sprintf("%s:%d", cfg.GrpcHost, cfg.GrpcPort),
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	)
	if err != nil {
		l.Fatal("failed to register api gateway handler from endpoint", zap.Error(err))
		return
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/docs/", http.StripPrefix("/docs", swaggerui.Handler(spec)))
	httpMux.Handle("/", gwmux)
	httpServer := httpserver.New(
		httpserver.AllowCors(httpMux),
		httpserver.Port(fmt.Sprintf("%d", cfg.HtpPort)),
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
