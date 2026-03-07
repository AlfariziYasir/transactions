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
	"github.com/AlfariziYasir/transactions/common/proto/order"
	"github.com/AlfariziYasir/transactions/common/proto/payment"
	"github.com/AlfariziYasir/transactions/services/order/config"
	"github.com/AlfariziYasir/transactions/services/order/internal/adapters/handler"
	"github.com/AlfariziYasir/transactions/services/order/internal/adapters/repository"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/services"
	"github.com/AlfariziYasir/transactions/services/order/migrations"
	"github.com/flowchartsman/swaggerui"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

//go:embed order_api.swagger.json
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
				"description": "handles all in and out event at order service",
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

	orderRepo := repository.NewOrderRepo(pg.Pool)
	productRepo := repository.NewProductRepo(pg.Pool)
	outboxRepo := repository.NewOutboxRepo(pg.Pool)
	inboxRepo := repository.NewInboxRepo(pg.Pool)
	trx := postgres.NewTransaction(pg.Pool)

	// grpc client
	paymentConn, err := grpc.NewClient(
		cfg.PaymentClient,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		l.Fatal("failed to connect client payment", zap.Error(err))
		return
	}
	defer paymentConn.Close()

	paymentClient := payment.NewPaymentServiceClient(paymentConn)

	svc := services.NewServices(orderRepo, productRepo, outboxRepo, inboxRepo, paymentClient, l, trx)

	productCh, err := rmqConn.Channel()
	if err != nil {
		l.Fatal("channel failed", zap.Error(err))
		return
	}
	defer productCh.Close()

	go func() {
		err = handler.NewProductConsumer(productRepo, inboxRepo, l, productCh, trx).Start()
		if err != nil {
			l.Fatal("failed to start product consumer", zap.Error(err))
			return
		}
	}()

	orderCh, err := rmqConn.Channel()
	if err != nil {
		l.Fatal("channel failed", zap.Error(err))
		return
	}
	defer orderCh.Close()

	err = handler.NewOrderConsumer(svc, inboxRepo, l, orderCh).Start()
	if err != nil {
		l.Fatal("failed to start order consumer", zap.Error(err))
		return
	}

	outboxCh, err := rmqConn.Channel()
	if err != nil {
		l.Fatal("channel failed", zap.Error(err))
		return
	}
	defer outboxCh.Close()

	pub, err := handler.NewPublisher(outboxRepo, outboxCh, l)
	if err != nil {
		l.Fatal("failed start publisher", zap.Error(err))
		return
	}
	go pub.Start(ctx)

	orderHandler := handler.NewHandler(svc, l)
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
	defer grpcServer.Shutdown()

	order.RegisterOrderServiceServer(grpcServer.Server, orderHandler)
	reflection.Register(grpcServer.Server)
	if err = grpcServer.Start(); err != nil {
		l.Fatal("failed to start gateway grpcServer", zap.Error(err))
		return
	}
	l.Info("GRPC server started")

	// start http server
	httpMux := http.NewServeMux()
	gwmux := grpcserver.RuntimeServer()
	err = order.RegisterOrderServiceHandlerFromEndpoint(
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
	defer func() {
		if err = httpServer.Shutdown(); err != nil {
			l.Fatal("failed to shutdown http server", zap.Error(err))
			return
		}
	}()
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
	grpcServer.Shutdown()
	httpServer.Shutdown()
	cancel()
	l.Info("server exited properly")
}
