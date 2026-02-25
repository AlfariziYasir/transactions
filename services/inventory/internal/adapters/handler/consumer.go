package handler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/logger"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/ports"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type stockConsumer struct {
	svc ports.StockService
	ch  *amqp091.Channel
	log *logger.Logger
}

func NewStockConsumer(
	svc ports.StockService,
	ch *amqp091.Channel,
	log *logger.Logger,
) *stockConsumer {
	return &stockConsumer{
		svc: svc,
		ch:  ch,
		log: log,
	}
}

func (c *stockConsumer) Start() error {
	// setup dead letter exchange
	err := c.ch.ExchangeDeclare("order.dlx", "direct", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed exchange declare", zap.Error(err))
		return err
	}

	qDlx, err := c.ch.QueueDeclare("inventory.status.dlq", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed queue declare", zap.Error(err))
		return err
	}

	err = c.ch.QueueBind(qDlx.Name, "status.failed", "inventory.dlx", false, nil)
	if err != nil {
		c.log.Error("failed queue bind", zap.Error(err))
		return err
	}

	// exchange declare procedure
	err = c.ch.ExchangeDeclare("order.events", "topic", true, false, false, false, nil)
	if err != nil {
		c.log.Error("failed exchange declare", zap.Error(err))
		return err
	}

	// setup main queue
	args := amqp091.Table{
		"x-dead-letter-exchange":    "inventory.dlx",
		"x-dead-letter-routing-key": "status.failed",
	}
	q, err := c.ch.QueueDeclare(
		"inventory.stock.update.queue",
		true, false, false, false, args,
	)
	if err != nil {
		c.log.Error("failed queue declare", zap.Error(err))
		return err
	}

	// binding queue to many exchanges
	err = c.ch.QueueBind(q.Name, "order.*", "order.events", false, nil)
	if err != nil {
		c.log.Error("failed queue bind", zap.Error(err))
		return err
	}

	// start consume
	msgs, err := c.ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		c.log.Error("failed consume", zap.Error(err))
		return err
	}

	c.log.Info("inventory status consumer started")
	go c.worker(msgs)
	return nil
}

func (c *stockConsumer) worker(msgs <-chan amqp091.Delivery) {
	for msg := range msgs {
		c.processMessage(msg)
	}
}

func (c *stockConsumer) processMessage(msg amqp091.Delivery) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var event model.OrderEvent
	err := json.Unmarshal(msg.Body, &event)
	if err != nil {
		c.log.Error("invalid json payload", zap.Error(err))
		return
	}

	switch msg.RoutingKey {
	case "order.created":
		err = c.svc.Reserve(ctx, &event)
	case "order.canceled":
		err = c.svc.Release(ctx, &event)
	case "order.paid":
		err = c.svc.Deduct(ctx, &event)
	}

	if err != nil {
		appErr, ok := errors.AsType[*errorx.AppError](err)
		if ok && appErr.Type == errorx.ErrTypeInternal {
			c.log.Error("invalid payload format, routing to DLX", zap.Error(err))
			msg.Nack(false, false)
			return
		}

		c.log.Error("failed to process message, requeueing", zap.Error(err))
		msg.Nack(false, true)
		return
	}

	c.log.Info("inventory status successfully updated via event", zap.String("event", msg.RoutingKey))
	msg.Ack(false)
}
