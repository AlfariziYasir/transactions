package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/httpx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/common/pkg/middleware"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/common/pkg/redis"
	"github.com/AlfariziYasir/transactions/common/proto/order"
	"github.com/AlfariziYasir/transactions/services/order/config"
	"github.com/AlfariziYasir/transactions/services/order/internal/adapters/handler"
	"github.com/AlfariziYasir/transactions/services/order/internal/adapters/repository"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/services"
	"github.com/AlfariziYasir/transactions/services/order/migrations"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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
	trx := postgres.NewTransaction(pg.Pool)

	svc := services.NewServices(
		orderRepo,
		productRepo,
		outboxRepo,
		l,
		trx,
	)

	productCh, err := rmqConn.Channel()
	if err != nil {
		l.Fatal("channel failed", zap.Error(err))
		return
	}
	defer productCh.Close()

	err = handler.NewProductConsumer(productRepo, l, productCh).Start()
	if err != nil {
		l.Fatal("failed to start product consumer", zap.Error(err))
		return
	}

	orderCh, err := rmqConn.Channel()
	if err != nil {
		l.Fatal("channel failed", zap.Error(err))
		return
	}
	defer orderCh.Close()

	err = handler.NewOrderConsumer(svc, l, orderCh).Start()
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

	go handler.NewPublisher(outboxRepo, outboxCh, l).Start(ctx)

	orderHandler := handler.NewHandler(svc, l)

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

		order.RegisterOrderServiceServer(grpcServer, orderHandler)
		reflection.Register(grpcServer)
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
			order.RegisterOrderServiceHandlerFromEndpoint,
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
